//go:build windows

package sandbox

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowsProcThreadAttributeSecurityCapabilities = 0x00020009
	windowsProcThreadAttributeJobList              = 0x0002000d
	windowsHResultAlreadyExists                    = 0x800700b7
)

var (
	windowsUserEnvDLL                    = windows.NewLazySystemDLL("userenv.dll")
	windowsCreateAppContainerProfileProc = windowsUserEnvDLL.NewProc("CreateAppContainerProfile")
	windowsDeriveAppContainerSIDProc     = windowsUserEnvDLL.NewProc("DeriveAppContainerSidFromAppContainerName")
	windowsDeleteAppContainerProfileProc = windowsUserEnvDLL.NewProc("DeleteAppContainerProfile")
	windowsGetAppContainerFolderPathProc = windowsUserEnvDLL.NewProc("GetAppContainerFolderPath")
	windowsCoTaskMemFreeProc             = windows.NewLazySystemDLL("ole32.dll").NewProc("CoTaskMemFree")
)

// windowsSecurityCapabilities mirrors SECURITY_CAPABILITIES. Leaving
// Capabilities nil/zero creates an AppContainer process without network
// capabilities; network is therefore denied by the AppContainer boundary.
type windowsSecurityCapabilities struct {
	AppContainerSID *windows.SID
	Capabilities    *windows.SIDAndAttributes
	CapabilityCount uint32
	Reserved        uint32
}

// createWindowsAppContainerProfile creates an ephemeral per-session
// AppContainer profile and returns a Go-owned copy of its package SID. The
// caller owns the profile name and must delete it when the sandbox is destroyed.
func createWindowsAppContainerProfile(name string) (*windows.SID, error) {
	if name == "" || len(name) > 64 {
		return nil, fmt.Errorf("invalid AppContainer profile name")
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("encode AppContainer name: %w", err)
	}
	displayPtr, err := windows.UTF16PtrFromString("OmniLLM Studio Sandbox")
	if err != nil {
		return nil, fmt.Errorf("encode AppContainer display name: %w", err)
	}
	descriptionPtr, err := windows.UTF16PtrFromString("Ephemeral OmniLLM Studio agent sandbox")
	if err != nil {
		return nil, fmt.Errorf("encode AppContainer description: %w", err)
	}

	var rawSID *windows.SID
	hr, _, _ := windowsCreateAppContainerProfileProc.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(displayPtr)),
		uintptr(unsafe.Pointer(descriptionPtr)),
		0,
		0,
		uintptr(unsafe.Pointer(&rawSID)),
	)
	if windowsHRESULTFailed(hr) {
		if uint32(hr) != windowsHResultAlreadyExists {
			return nil, windowsHRESULTError("CreateAppContainerProfile", hr)
		}
		hr, _, _ = windowsDeriveAppContainerSIDProc.Call(
			uintptr(unsafe.Pointer(namePtr)),
			uintptr(unsafe.Pointer(&rawSID)),
		)
		if windowsHRESULTFailed(hr) {
			return nil, windowsHRESULTError("DeriveAppContainerSidFromAppContainerName", hr)
		}
	}
	if rawSID == nil || !rawSID.IsValid() {
		return nil, fmt.Errorf("AppContainer profile returned an invalid SID")
	}
	defer windows.FreeSid(rawSID)
	copied, err := rawSID.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy AppContainer SID: %w", err)
	}
	return copied, nil
}

// windowsAppContainerFolderPath returns the AppContainer's own local app-data
// folder. The OS creates and ACLs this profile directory for the package SID,
// making it the narrowest place to stage sandbox-owned workspace/home/tmp data.
func windowsAppContainerFolderPath(sid *windows.SID) (string, error) {
	if sid == nil || !sid.IsValid() {
		return "", fmt.Errorf("valid AppContainer SID is required")
	}
	sidString := sid.String()
	if sidString == "" {
		return "", fmt.Errorf("format AppContainer SID")
	}
	sidPtr, err := windows.UTF16PtrFromString(sidString)
	if err != nil {
		return "", fmt.Errorf("encode AppContainer SID: %w", err)
	}
	var rawPath *uint16
	hr, _, _ := windowsGetAppContainerFolderPathProc.Call(
		uintptr(unsafe.Pointer(sidPtr)),
		uintptr(unsafe.Pointer(&rawPath)),
	)
	if windowsHRESULTFailed(hr) {
		return "", windowsHRESULTError("GetAppContainerFolderPath", hr)
	}
	if rawPath == nil {
		return "", fmt.Errorf("GetAppContainerFolderPath returned an empty path")
	}
	defer windowsCoTaskMemFreeProc.Call(uintptr(unsafe.Pointer(rawPath)))
	path := windows.UTF16PtrToString(rawPath)
	if path == "" {
		return "", fmt.Errorf("GetAppContainerFolderPath returned an empty path")
	}
	return path, nil
}

// deleteWindowsAppContainerProfile is idempotent at the Windows API boundary;
// DeleteAppContainerProfile succeeds when the named profile no longer exists.
func deleteWindowsAppContainerProfile(name string) error {
	if name == "" {
		return nil
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("encode AppContainer name: %w", err)
	}
	hr, _, _ := windowsDeleteAppContainerProfileProc.Call(uintptr(unsafe.Pointer(namePtr)))
	if windowsHRESULTFailed(hr) {
		return windowsHRESULTError("DeleteAppContainerProfile", hr)
	}
	return nil
}

func windowsHRESULTFailed(value uintptr) bool {
	return int32(uint32(value)) < 0
}

func windowsHRESULTError(operation string, value uintptr) error {
	return fmt.Errorf("%s failed with HRESULT 0x%08x", operation, uint32(value))
}
