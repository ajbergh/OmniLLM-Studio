//go:build windows

package sandbox

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// startWindowsExtensionAppContainerProcess applies the AppContainer identity,
// Job Object, and explicit stdio handle list in the process creation attribute
// list. The extension therefore never runs first under the unrestricted backend
// token and cannot escape before Job membership is established.
func startWindowsExtensionAppContainerProcess(
	application string,
	args []string,
	workingDirectory string,
	environment []uint16,
	appSID *windows.SID,
	job windows.Handle,
	pipes *windowsSandboxPipes,
) (windows.ProcessInformation, error) {
	attributes, err := windows.NewProcThreadAttributeList(3)
	if err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("create Windows extension process attribute list: %w", err)
	}
	defer attributes.Delete()

	securityCapabilities := windowsSecurityCapabilities{AppContainerSID: appSID}
	if err := attributes.Update(
		windowsProcThreadAttributeSecurityCapabilities,
		unsafe.Pointer(&securityCapabilities),
		unsafe.Sizeof(securityCapabilities),
	); err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("attach AppContainer security capabilities: %w", err)
	}
	jobs := []windows.Handle{job}
	if err := attributes.Update(
		windowsProcThreadAttributeJobList,
		unsafe.Pointer(&jobs[0]),
		unsafe.Sizeof(jobs[0]),
	); err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("attach Job Object at process creation: %w", err)
	}
	handles := []windows.Handle{pipes.childStdin, pipes.childStdout, pipes.childStderr}
	if err := attributes.Update(
		windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		unsafe.Pointer(&handles[0]),
		uintptr(len(handles))*unsafe.Sizeof(handles[0]),
	); err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("restrict inherited extension process handles: %w", err)
	}

	applicationPtr, err := windows.UTF16PtrFromString(application)
	if err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("encode extension executable path: %w", err)
	}
	commandLineUTF16, err := windows.UTF16FromString(windowsCommandLine(application, args))
	if err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("encode extension command line: %w", err)
	}
	workingDirectoryPtr, err := windows.UTF16PtrFromString(workingDirectory)
	if err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("encode extension working directory: %w", err)
	}
	if len(environment) == 0 {
		return windows.ProcessInformation{}, fmt.Errorf("Windows extension environment block is empty")
	}

	startup := windows.StartupInfoEx{}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	startup.Flags = windows.STARTF_USESTDHANDLES
	startup.StdInput = pipes.childStdin
	startup.StdOutput = pipes.childStdout
	startup.StdErr = pipes.childStderr
	startup.ProcThreadAttributeList = attributes.List()
	processInfo := windows.ProcessInformation{}
	creationFlags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_NO_WINDOW)
	if err := windows.CreateProcess(
		applicationPtr,
		&commandLineUTF16[0],
		nil,
		nil,
		true,
		creationFlags,
		&environment[0],
		workingDirectoryPtr,
		&startup.StartupInfo,
		&processInfo,
	); err != nil {
		return windows.ProcessInformation{}, fmt.Errorf("create Windows AppContainer extension process: %w", err)
	}
	return processInfo, nil
}
