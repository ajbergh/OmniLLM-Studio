//go:build windows

package sandbox

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsDisableMaxPrivilege = 0x1

var windowsCreateRestrictedTokenProc = windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")

// createWindowsSandboxSID returns an unregistered, random SID used only as a
// restricting identity for one sandbox. A per-sandbox SID prevents a filesystem
// ACE granted to one sandbox from authorizing another restricted process owned
// by the same Windows user.
func createWindowsSandboxSID() (*windows.SID, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return nil, fmt.Errorf("generate sandbox SID entropy: %w", err)
	}
	sid, err := windows.StringToSid(fmt.Sprintf(
		"S-1-5-21-%d-%d-%d-%d",
		binary.LittleEndian.Uint32(entropy[0:4]),
		binary.LittleEndian.Uint32(entropy[4:8]),
		binary.LittleEndian.Uint32(entropy[8:12]),
		binary.LittleEndian.Uint32(entropy[12:16]),
	))
	if err != nil {
		return nil, fmt.Errorf("create sandbox SID: %w", err)
	}
	return sid, nil
}

// createWindowsRestrictedToken creates a primary token derived from the current
// process token with privileges disabled and exactly one application-issued
// restricting SID. Filesystem access by this token must pass both the user's
// normal access check and the sandbox-specific restricting-SID access check.
func createWindowsRestrictedToken(restrictingSID *windows.SID) (windows.Token, error) {
	if restrictingSID == nil || !restrictingSID.IsValid() {
		return 0, fmt.Errorf("valid sandbox restricting SID is required")
	}
	return createWindowsFilteredToken(restrictingSID)
}

// createWindowsPrivilegeStrippedToken derives a primary token with maximum
// privileges disabled but without a process-wide restricting-SID access check.
// The Windows AppContainer runtime uses this as its base token: AppContainer's
// package SID provides the unique filesystem/network boundary, while avoiding a
// restricting SID that would also need execute/read ACEs on every system DLL.
func createWindowsPrivilegeStrippedToken() (windows.Token, error) {
	return createWindowsFilteredToken(nil)
}

func createWindowsFilteredToken(restrictingSID *windows.SID) (windows.Token, error) {
	var source windows.Token
	access := uint32(windows.TOKEN_DUPLICATE | windows.TOKEN_QUERY | windows.TOKEN_ASSIGN_PRIMARY)
	if err := windows.OpenProcessToken(windows.CurrentProcess(), access, &source); err != nil {
		return 0, fmt.Errorf("open current process token: %w", err)
	}
	defer source.Close()

	var restrictedCount uintptr
	var restrictedPointer uintptr
	var restricting []windows.SIDAndAttributes
	if restrictingSID != nil {
		restricting = []windows.SIDAndAttributes{{Sid: restrictingSID}}
		restrictedCount = uintptr(len(restricting))
		restrictedPointer = uintptr(unsafe.Pointer(&restricting[0]))
	}

	var restricted windows.Token
	result, _, callErr := windowsCreateRestrictedTokenProc.Call(
		uintptr(source),
		uintptr(windowsDisableMaxPrivilege),
		0,
		0,
		0,
		0,
		restrictedCount,
		restrictedPointer,
		uintptr(unsafe.Pointer(&restricted)),
	)
	runtime.KeepAlive(restrictingSID)
	runtime.KeepAlive(restricting)
	if result == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return 0, fmt.Errorf("CreateRestrictedToken: %w", callErr)
		}
		return 0, fmt.Errorf("CreateRestrictedToken failed")
	}
	return restricted, nil
}

// createWindowsKillOnCloseJob creates a Job Object whose process tree is
// terminated when the final job handle is closed. Process assignment is kept
// separate so later runtime integration can bind children at creation time and
// avoid an escape window before assignment.
func createWindowsKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, fmt.Errorf("configure kill-on-close job object: %w", err)
	}
	return job, nil
}

// grantWindowsSIDAccess merges an allow ACE for one sandbox-specific SID into
// path's existing DACL. It preserves the existing identity DACL: a restricted
// token must satisfy both the normal user access check and this sandbox-specific
// restricting-SID check.
func grantWindowsSIDAccess(path string, sid *windows.SID, access windows.ACCESS_MASK, inherit bool) error {
	if sid == nil || !sid.IsValid() {
		return fmt.Errorf("valid sandbox SID is required")
	}

	current, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read DACL for %q: %w", path, err)
	}
	var existing *windows.ACL
	if current != nil {
		existing, _, err = current.DACL()
		if err != nil {
			return fmt.Errorf("read existing DACL for %q: %w", path, err)
		}
	}

	var pinner runtime.Pinner
	pinner.Pin(sid)
	defer pinner.Unpin()

	inheritance := uint32(windows.NO_INHERITANCE)
	if inherit {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entry := windows.EXPLICIT_ACCESS{
		AccessPermissions: access,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
	merged, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, existing)
	if err != nil {
		return fmt.Errorf("merge sandbox DACL for %q: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil,
		nil,
		merged,
		nil,
	); err != nil {
		return fmt.Errorf("write sandbox DACL for %q: %w", path, err)
	}
	return nil
}
