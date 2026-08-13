//go:build windows

package sandbox

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

// setWindowsAppContainerDirectoryAccess replaces a sandbox-owned directory DACL
// before child content is created. The backend user, LocalSystem, and local
// Administrators retain full control for cleanup; the AppContainer package SID
// receives only the requested access. Protecting the DACL prevents the profile
// directory's broader inherited package ACEs from silently widening a staged
// read-only workspace.
func setWindowsAppContainerDirectoryAccess(path string, appContainerSID *windows.SID, appAccess windows.ACCESS_MASK) error {
	if appContainerSID == nil || !appContainerSID.IsValid() {
		return fmt.Errorf("valid AppContainer SID is required")
	}
	userInfo, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("query current token user: %w", err)
	}
	userSID, err := userInfo.User.Sid.Copy()
	if err != nil {
		return fmt.Errorf("copy current user SID: %w", err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("create LocalSystem SID: %w", err)
	}
	administratorsSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("create Administrators SID: %w", err)
	}

	fullControl := windows.ACCESS_MASK(windows.GENERIC_ALL)
	inheritance := uint32(windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT)
	entries := []windows.EXPLICIT_ACCESS{
		windowsSIDAccessEntry(userSID, fullControl, inheritance),
		windowsSIDAccessEntry(systemSID, fullControl, inheritance),
		windowsSIDAccessEntry(administratorsSID, fullControl, inheritance),
		windowsSIDAccessEntry(appContainerSID, appAccess, inheritance),
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build protected AppContainer DACL for %q: %w", path, err)
	}
	securityInfo := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, securityInfo, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("write protected AppContainer DACL for %q: %w", path, err)
	}
	runtime.KeepAlive(userSID)
	runtime.KeepAlive(systemSID)
	runtime.KeepAlive(administratorsSID)
	runtime.KeepAlive(appContainerSID)
	return nil
}

func windowsSIDAccessEntry(sid *windows.SID, access windows.ACCESS_MASK, inheritance uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: access,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}
