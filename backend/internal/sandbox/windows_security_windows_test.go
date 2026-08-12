//go:build windows

package sandbox

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsRestrictedTokenIsRestricted(t *testing.T) {
	token, err := createWindowsRestrictedToken()
	if err != nil {
		t.Fatal(err)
	}
	defer token.Close()

	restricted, err := token.IsRestricted()
	if err != nil {
		t.Fatalf("query restricted token: %v", err)
	}
	if !restricted {
		t.Fatal("CreateRestrictedToken result is not reported as restricted")
	}
}

func TestWindowsKillOnCloseJobFlag(t *testing.T) {
	job, err := createWindowsKillOnCloseJob()
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	var returned uint32
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		&returned,
	); err != nil {
		t.Fatalf("query job object limits: %v", err)
	}
	if info.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
		t.Fatalf("job limit flags %#x do not include KILL_ON_JOB_CLOSE", info.BasicLimitInformation.LimitFlags)
	}
}

func TestWindowsRestrictedTokenACLScopesWrites(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	if err := os.Mkdir(allowed, 0o755); err != nil {
		t.Fatal(err)
	}

	readTraverse := windows.ACCESS_MASK(
		windows.FILE_GENERIC_READ |
			windows.FILE_GENERIC_EXECUTE |
			windows.FILE_LIST_DIRECTORY |
			windows.FILE_TRAVERSE,
	)
	readWriteTraverse := windows.ACCESS_MASK(
		windows.FILE_GENERIC_READ |
			windows.FILE_GENERIC_WRITE |
			windows.FILE_GENERIC_EXECUTE |
			windows.FILE_LIST_DIRECTORY |
			windows.FILE_TRAVERSE,
	)
	if err := grantWindowsRestrictedCodeAccess(root, readTraverse, false); err != nil {
		t.Fatalf("grant root traversal: %v", err)
	}
	if err := grantWindowsRestrictedCodeAccess(allowed, readWriteTraverse, true); err != nil {
		t.Fatalf("grant allowed workspace access: %v", err)
	}

	primary, err := createWindowsRestrictedToken()
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()

	var impersonation windows.Token
	if err := windows.DuplicateTokenEx(
		primary,
		windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE,
		nil,
		windows.SecurityImpersonation,
		windows.TokenImpersonation,
		&impersonation,
	); err != nil {
		t.Fatalf("duplicate impersonation token: %v", err)
	}
	defer impersonation.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.SetThreadToken(nil, impersonation); err != nil {
		t.Fatalf("impersonate restricted token: %v", err)
	}
	defer func() {
		if err := windows.RevertToSelf(); err != nil {
			t.Errorf("revert restricted impersonation: %v", err)
		}
	}()

	if err := os.WriteFile(filepath.Join(allowed, "inside.txt"), []byte("allowed"), 0o644); err != nil {
		t.Fatalf("restricted token could not write inside explicitly granted workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "blocked.txt"), []byte("blocked"), 0o644); err == nil {
		t.Fatal("restricted token unexpectedly wrote outside explicitly writable workspace")
	} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("outside-workspace write failed with %v; want access denied", err)
	}
}
