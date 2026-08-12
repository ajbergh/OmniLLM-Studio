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
	sid, err := createWindowsSandboxSID()
	if err != nil {
		t.Fatal(err)
	}
	token, err := createWindowsRestrictedToken(sid)
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
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), &returned); err != nil {
		t.Fatalf("query job object limits: %v", err)
	}
	if info.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
		t.Fatalf("job limit flags %#x do not include KILL_ON_JOB_CLOSE", info.BasicLimitInformation.LimitFlags)
	}
}

func TestWindowsSandboxSIDACLScopesAndSeparatesWrites(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	if err := os.Mkdir(allowed, 0o755); err != nil {
		t.Fatal(err)
	}
	sidA, err := createWindowsSandboxSID()
	if err != nil {
		t.Fatal(err)
	}
	sidB, err := createWindowsSandboxSID()
	if err != nil {
		t.Fatal(err)
	}
	if sidA.String() == sidB.String() {
		t.Fatal("sandbox SIDs must be unique")
	}

	readTraverse := windows.ACCESS_MASK(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE | windows.FILE_LIST_DIRECTORY | windows.FILE_TRAVERSE)
	readWriteTraverse := windows.ACCESS_MASK(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE | windows.FILE_GENERIC_EXECUTE | windows.FILE_LIST_DIRECTORY | windows.FILE_TRAVERSE)
	for _, sid := range []*windows.SID{sidA, sidB} {
		if err := grantWindowsSIDAccess(root, sid, readTraverse, false); err != nil {
			t.Fatalf("grant root traversal: %v", err)
		}
	}
	if err := grantWindowsSIDAccess(allowed, sidA, readWriteTraverse, true); err != nil {
		t.Fatalf("grant sandbox A workspace access: %v", err)
	}

	tokenA, err := createWindowsRestrictedToken(sidA)
	if err != nil {
		t.Fatal(err)
	}
	defer tokenA.Close()
	tokenB, err := createWindowsRestrictedToken(sidB)
	if err != nil {
		t.Fatal(err)
	}
	defer tokenB.Close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	impersonate := func(token windows.Token, fn func()) {
		var impersonation windows.Token
		if err := windows.DuplicateTokenEx(token, windows.TOKEN_QUERY|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &impersonation); err != nil {
			t.Fatalf("duplicate impersonation token: %v", err)
		}
		defer impersonation.Close()
		if err := windows.SetThreadToken(nil, impersonation); err != nil {
			t.Fatalf("impersonate restricted token: %v", err)
		}
		defer func() {
			if err := windows.RevertToSelf(); err != nil {
				t.Fatalf("revert restricted impersonation: %v", err)
			}
		}()
		fn()
	}

	impersonate(tokenA, func() {
		if err := os.WriteFile(filepath.Join(allowed, "inside-a.txt"), []byte("allowed"), 0o644); err != nil {
			t.Fatalf("sandbox A could not write inside its workspace: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "blocked.txt"), []byte("blocked"), 0o644); err == nil {
			t.Fatal("sandbox A unexpectedly wrote outside its workspace")
		} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("outside-workspace write failed with %v; want access denied", err)
		}
	})
	impersonate(tokenB, func() {
		if err := os.WriteFile(filepath.Join(allowed, "inside-b.txt"), []byte("blocked"), 0o644); err == nil {
			t.Fatal("sandbox B unexpectedly reused sandbox A workspace ACL")
		} else if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("cross-sandbox write failed with %v; want access denied", err)
		}
	})
}
