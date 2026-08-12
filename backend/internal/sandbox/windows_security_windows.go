//go:build windows

package sandbox

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsDisableMaxPrivilege = 0x1

var windowsCreateRestrictedTokenProc = windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")

// createWindowsRestrictedToken creates a primary token derived from the current
// process token with privileges disabled and the Restricted Code SID added to
// its restricting SID list. Filesystem access by this token therefore has to
// pass both the normal token access check and the Restricted Code access check.
func createWindowsRestrictedToken() (windows.Token, error) {
	var source windows.Token
	access := uint32(windows.TOKEN_DUPLICATE | windows.TOKEN_QUERY | windows.TOKEN_ASSIGN_PRIMARY)
	if err := windows.OpenProcessToken(windows.CurrentProcess(), access, &source); err != nil {
		return 0, fmt.Errorf("open current process token: %w", err)
	}
	defer source.Close()

	restrictedCodeSID, err := windows.CreateWellKnownSid(windows.WinRestrictedCodeSid)
	if err != nil {
		return 0, fmt.Errorf("create Restricted Code SID: %w", err)
	}
	restricting := []windows.SIDAndAttributes{{Sid: restrictedCodeSID}}

	var restricted windows.Token
	result, _, callErr := windowsCreateRestrictedTokenProc.Call(
		uintptr(source),
		uintptr(windowsDisableMaxPrivilege),
		0,
		0,
		0,
		0,
		uintptr(len(restricting)),
		uintptr(unsafe.Pointer(&restricting[0])),
		uintptr(unsafe.Pointer(&restricted)),
	)
	runtime.KeepAlive(restrictedCodeSID)
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

// grantWindowsRestrictedCodeAccess merges an allow ACE for the Restricted Code
// SID into path's existing DACL. It deliberately preserves the existing DACL:
// a restricted token must satisfy both the original identity permissions and
// the Restricted Code permissions rather than replacing the user's ACL.
func grantWindowsRestrictedCodeAccess(path string, access windows.ACCESS_MASK, inherit bool) error {
	restrictedCodeSID, err := windows.CreateWellKnownSid(windows.WinRestrictedCodeSid)
	if err != nil {
		return fmt.Errorf("create Restricted Code SID: %w", err)
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
	pinner.Pin(restrictedCodeSID)
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
			TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(restrictedCodeSID),
		},
	}
	merged, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{entry}, existing)
	if err != nil {
		return fmt.Errorf("merge Restricted Code DACL for %q: %w", path, err)
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
		return fmt.Errorf("write Restricted Code DACL for %q: %w", path, err)
	}
	return nil
}
