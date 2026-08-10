package gitrepo

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5"
)

var (
	errCloneStorageQuotaExceeded = errors.New("clone storage quota exceeded")
	errCloneEntryQuotaExceeded   = errors.New("clone entry quota exceeded")
)

// cloneQuota is shared by the worktree and .git filesystems for one clone. It
// bounds logical bytes written/expanded and the cumulative number of filesystem
// entries created. Removals do not refund quota; conservative over-counting is
// intentional so retries and partial failures cannot create quota credit.
type cloneQuota struct {
	mu sync.Mutex

	maxBytes    int64
	maxEntries  int64
	usedBytes   int64
	usedEntries int64
}

func newCloneQuota(maxBytes, maxEntries int64) *cloneQuota {
	return &cloneQuota{maxBytes: maxBytes, maxEntries: maxEntries}
}

func (q *cloneQuota) reserveLocked(bytes, entries int64) error {
	if bytes < 0 || entries < 0 {
		return errCloneStorageQuotaExceeded
	}
	if bytes > 0 {
		if q.maxBytes <= 0 || bytes > q.maxBytes-q.usedBytes {
			return errCloneStorageQuotaExceeded
		}
	}
	if entries > 0 {
		if q.maxEntries <= 0 || entries > q.maxEntries-q.usedEntries {
			return errCloneEntryQuotaExceeded
		}
	}
	q.usedBytes += bytes
	q.usedEntries += entries
	return nil
}

func (q *cloneQuota) reserve(bytes, entries int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.reserveLocked(bytes, entries)
}

func (q *cloneQuota) releaseBytes(bytes int64) {
	if bytes <= 0 {
		return
	}
	q.usedBytes -= bytes
	if q.usedBytes < 0 {
		q.usedBytes = 0
	}
}

func (q *cloneQuota) usage() (bytes, entries int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.usedBytes, q.usedEntries
}

// cloneQuotaFilesystem wraps a billy filesystem and shares one quota across all
// chroots. The wrapper is used for both the temporary worktree and .git storage,
// so pack ingestion, indexes/config, checkout data, symlink targets, and sparse
// logical expansion all consume the same budget before atomic promotion.
type cloneQuotaFilesystem struct {
	base  billy.Filesystem
	quota *cloneQuota
}

func newCloneQuotaFilesystem(base billy.Filesystem, maxBytes, maxEntries int64) *cloneQuotaFilesystem {
	return &cloneQuotaFilesystem{base: base, quota: newCloneQuota(maxBytes, maxEntries)}
}

func (fs *cloneQuotaFilesystem) Create(filename string) (billy.File, error) {
	if err := fs.reserveNewFile(filename); err != nil {
		return nil, err
	}
	file, err := fs.base.Create(filename)
	if err != nil {
		return nil, err
	}
	return &cloneQuotaFile{File: file, quota: fs.quota}, nil
}

func (fs *cloneQuotaFilesystem) Open(filename string) (billy.File, error) {
	file, err := fs.base.Open(filename)
	if err != nil {
		return nil, err
	}
	return &cloneQuotaFile{File: file, quota: fs.quota}, nil
}

func (fs *cloneQuotaFilesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if flag&os.O_CREATE != 0 {
		if err := fs.reserveNewFile(filename); err != nil {
			return nil, err
		}
	}
	file, err := fs.base.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	return &cloneQuotaFile{File: file, quota: fs.quota, appendMode: flag&os.O_APPEND != 0}, nil
}

func (fs *cloneQuotaFilesystem) Stat(filename string) (os.FileInfo, error) {
	return fs.base.Stat(filename)
}

func (fs *cloneQuotaFilesystem) Rename(oldpath, newpath string) error {
	return fs.base.Rename(oldpath, newpath)
}

func (fs *cloneQuotaFilesystem) Remove(filename string) error {
	return fs.base.Remove(filename)
}

func (fs *cloneQuotaFilesystem) Join(elem ...string) string {
	return fs.base.Join(elem...)
}

func (fs *cloneQuotaFilesystem) TempFile(dir, prefix string) (billy.File, error) {
	if err := fs.quota.reserve(0, 1); err != nil {
		return nil, err
	}
	file, err := fs.base.TempFile(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &cloneQuotaFile{File: file, quota: fs.quota}, nil
}

func (fs *cloneQuotaFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	return fs.base.ReadDir(path)
}

func (fs *cloneQuotaFilesystem) MkdirAll(filename string, perm os.FileMode) error {
	missing, err := missingDirectoryEntries(fs.base, filename)
	if err != nil {
		return err
	}
	if err := fs.quota.reserve(0, missing); err != nil {
		return err
	}
	return fs.base.MkdirAll(filename, perm)
}

func (fs *cloneQuotaFilesystem) Lstat(filename string) (os.FileInfo, error) {
	return fs.base.Lstat(filename)
}

func (fs *cloneQuotaFilesystem) Symlink(target, link string) error {
	missing, err := missingDirectoryEntries(fs.base, filepath.Dir(link))
	if err != nil {
		return err
	}
	entries := missing
	if _, err := fs.base.Lstat(link); os.IsNotExist(err) {
		entries++
	} else if err != nil {
		return err
	}
	if err := fs.quota.reserve(int64(len(target)), entries); err != nil {
		return err
	}
	return fs.base.Symlink(target, link)
}

func (fs *cloneQuotaFilesystem) Readlink(link string) (string, error) {
	return fs.base.Readlink(link)
}

func (fs *cloneQuotaFilesystem) Chroot(path string) (billy.Filesystem, error) {
	child, err := fs.base.Chroot(path)
	if err != nil {
		return nil, err
	}
	return &cloneQuotaFilesystem{base: child, quota: fs.quota}, nil
}

func (fs *cloneQuotaFilesystem) Root() string {
	return fs.base.Root()
}

func (fs *cloneQuotaFilesystem) Capabilities() billy.Capability {
	return billy.Capabilities(fs.base)
}

func (fs *cloneQuotaFilesystem) Chmod(name string, mode os.FileMode) error {
	changer, ok := fs.base.(billy.Chmod)
	if !ok {
		return billy.ErrNotSupported
	}
	return changer.Chmod(name, mode)
}

func (fs *cloneQuotaFilesystem) Lchown(name string, uid, gid int) error {
	changer, ok := fs.base.(billy.Change)
	if !ok {
		return billy.ErrNotSupported
	}
	return changer.Lchown(name, uid, gid)
}

func (fs *cloneQuotaFilesystem) Chown(name string, uid, gid int) error {
	changer, ok := fs.base.(billy.Change)
	if !ok {
		return billy.ErrNotSupported
	}
	return changer.Chown(name, uid, gid)
}

func (fs *cloneQuotaFilesystem) Chtimes(name string, atime, mtime time.Time) error {
	changer, ok := fs.base.(billy.Change)
	if !ok {
		return billy.ErrNotSupported
	}
	return changer.Chtimes(name, atime, mtime)
}

func (fs *cloneQuotaFilesystem) reserveNewFile(filename string) error {
	_, err := fs.base.Lstat(filename)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return fs.quota.reserve(0, 1)
}

func missingDirectoryEntries(fs billy.Filesystem, filename string) (int64, error) {
	filename = filepath.Clean(filename)
	if filename == "." || filename == "" || filename == string(filepath.Separator) {
		return 0, nil
	}
	var missing int64
	current := filename
	for {
		info, err := fs.Lstat(current)
		if err == nil {
			if !info.IsDir() {
				return 0, &os.PathError{Op: "mkdir", Path: current, Err: errors.New("path component is not a directory")}
			}
			break
		}
		if !os.IsNotExist(err) {
			return 0, err
		}
		missing++
		parent := filepath.Dir(current)
		if parent == current || parent == "." || parent == "" {
			break
		}
		current = parent
	}
	return missing, nil
}

// cloneQuotaFile charges logical expansion before the underlying filesystem is
// allowed to write. Seeking far beyond EOF and then writing a single byte is
// charged for the full sparse logical expansion, closing a common quota bypass.
type cloneQuotaFile struct {
	billy.File
	quota      *cloneQuota
	appendMode bool
}

func (f *cloneQuotaFile) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return f.File.Write(p)
	}
	f.quota.mu.Lock()
	defer f.quota.mu.Unlock()

	current, size, err := currentOffsetAndSize(f.File)
	if err != nil {
		return 0, err
	}
	writeOffset := current
	if f.appendMode {
		writeOffset = size
	}
	if writeOffset < 0 || writeOffset > (1<<63-1)-int64(len(p)) {
		return 0, errCloneStorageQuotaExceeded
	}
	plannedEnd := writeOffset + int64(len(p))
	charge := int64(len(p))
	if plannedEnd > size && plannedEnd-size > charge {
		charge = plannedEnd - size
	}
	if err := f.quota.reserveLocked(charge, 0); err != nil {
		return 0, err
	}

	n, writeErr := f.File.Write(p)
	actualEnd := writeOffset + int64(n)
	actualCharge := int64(n)
	if actualEnd > size && actualEnd-size > actualCharge {
		actualCharge = actualEnd - size
	}
	if actualCharge < charge {
		f.quota.releaseBytes(charge - actualCharge)
	}
	return n, writeErr
}

func (f *cloneQuotaFile) Truncate(size int64) error {
	if size < 0 {
		return os.ErrInvalid
	}
	f.quota.mu.Lock()
	defer f.quota.mu.Unlock()

	current, existing, err := currentOffsetAndSize(f.File)
	if err != nil {
		return err
	}
	if size > existing {
		if err := f.quota.reserveLocked(size-existing, 0); err != nil {
			return err
		}
	}
	if err := f.File.Truncate(size); err != nil {
		// Deliberately keep the reservation on failure. Some filesystems may
		// partially mutate before returning an error; conservative over-counting
		// keeps the clone fail-closed instead of manufacturing quota credit.
		return err
	}
	_, _ = f.File.Seek(current, io.SeekStart)
	return nil
}

func currentOffsetAndSize(file billy.File) (current, size int64, err error) {
	current, err = file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, 0, err
	}
	size, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}
	if _, err = file.Seek(current, io.SeekStart); err != nil {
		return 0, 0, err
	}
	return current, size, nil
}
