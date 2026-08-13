//go:build windows

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

const windowsExtensionCleanupAttempts = 20

type windowsExtensionProcess struct {
	ctx  context.Context
	spec ProcessSpec
	plan windowsExtensionLaunchPlan

	mu sync.Mutex

	pipes       *windowsSandboxPipes
	stdinGiven  bool
	stdoutGiven bool
	stderrGiven bool
	stdout      *windowsExtensionReadPipe
	stderr      *windowsExtensionReadPipe

	started bool
	job     windows.Handle
	process windows.Handle

	profileName string
	profileRoot string

	waitOnce sync.Once
	waitDone chan struct{}
	waitErr  error
}

type windowsExtensionReadPipe struct {
	file *os.File
	once sync.Once
}

func newWindowsExtensionProcess(ctx context.Context, spec ProcessSpec, plan windowsExtensionLaunchPlan) CommandProcess {
	return &windowsExtensionProcess{
		ctx:      ctx,
		spec:     spec,
		plan:     plan,
		waitDone: make(chan struct{}),
	}
}

func (p *windowsExtensionReadPipe) Read(buffer []byte) (int, error) {
	if p == nil || p.file == nil {
		return 0, fmt.Errorf("Windows extension pipe is closed")
	}
	n, err := p.file.Read(buffer)
	if errors.Is(err, io.EOF) {
		_ = p.Close()
	}
	return n, err
}

func (p *windowsExtensionReadPipe) Close() error {
	if p == nil {
		return nil
	}
	var closeErr error
	p.once.Do(func() {
		if p.file != nil {
			closeErr = p.file.Close()
		}
	})
	return closeErr
}

func (p *windowsExtensionProcess) ensurePipesLocked() error {
	if p.pipes != nil {
		return nil
	}
	pipes, err := createWindowsSandboxPipes()
	if err != nil {
		return err
	}
	p.pipes = pipes
	p.stdout = &windowsExtensionReadPipe{file: pipes.parentStdout}
	p.stderr = &windowsExtensionReadPipe{file: pipes.parentStderr}
	pipes.parentStdout = nil
	pipes.parentStderr = nil
	return nil
}

func (p *windowsExtensionProcess) StdinPipe() (io.WriteCloser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil, fmt.Errorf("stdin pipe requested after process start")
	}
	if p.stdinGiven {
		return nil, fmt.Errorf("stdin pipe already requested")
	}
	if err := p.ensurePipesLocked(); err != nil {
		return nil, err
	}
	p.stdinGiven = true
	return p.pipes.parentStdin, nil
}

func (p *windowsExtensionProcess) StdoutPipe() (io.ReadCloser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil, fmt.Errorf("stdout pipe requested after process start")
	}
	if p.stdoutGiven {
		return nil, fmt.Errorf("stdout pipe already requested")
	}
	if err := p.ensurePipesLocked(); err != nil {
		return nil, err
	}
	p.stdoutGiven = true
	return p.stdout, nil
}

func (p *windowsExtensionProcess) StderrPipe() (io.ReadCloser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return nil, fmt.Errorf("stderr pipe requested after process start")
	}
	if p.stderrGiven {
		return nil, fmt.Errorf("stderr pipe already requested")
	}
	if err := p.ensurePipesLocked(); err != nil {
		return nil, err
	}
	p.stderrGiven = true
	return p.stderr, nil
}

func (p *windowsExtensionProcess) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return fmt.Errorf("process already started")
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if err := p.ensurePipesLocked(); err != nil {
		return err
	}

	profileName := "OmniLLM.Extension." + strings.ReplaceAll(uuid.NewString(), "-", "")
	appSID, err := createWindowsAppContainerProfile(profileName)
	if err != nil {
		p.closePipesLocked()
		return err
	}
	profileRoot, err := windowsAppContainerFolderPath(appSID)
	if err != nil {
		_ = deleteWindowsAppContainerProfile(profileName)
		p.closePipesLocked()
		return err
	}
	cleanupProfile := true
	defer func() {
		if cleanupProfile {
			_ = cleanupWindowsExtensionProfile(profileName, profileRoot)
		}
	}()

	extensionDir := filepath.Join(profileRoot, "extension")
	workspaceDir := filepath.Join(profileRoot, "workspace")
	homeDir := filepath.Join(profileRoot, "home")
	tmpDir := filepath.Join(profileRoot, "tmp")
	for _, directory := range []string{extensionDir, workspaceDir, homeDir, tmpDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			p.closePipesLocked()
			return fmt.Errorf("create Windows extension sandbox directory %q: %w", directory, err)
		}
	}

	readOnlyAccess := windows.ACCESS_MASK(windows.FILE_GENERIC_READ | windows.FILE_GENERIC_EXECUTE | windows.FILE_LIST_DIRECTORY | windows.FILE_TRAVERSE)
	readWriteAccess := windows.ACCESS_MASK(windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.FILE_GENERIC_EXECUTE|windows.FILE_LIST_DIRECTORY|windows.FILE_TRAVERSE|windows.DELETE) | windowsFileDeleteChild
	for _, access := range []struct {
		path string
		mask windows.ACCESS_MASK
	}{{extensionDir, readOnlyAccess}, {workspaceDir, readOnlyAccess}, {homeDir, readWriteAccess}, {tmpDir, readWriteAccess}} {
		if err := setWindowsAppContainerDirectoryAccess(access.path, appSID, access.mask); err != nil {
			p.closePipesLocked()
			return err
		}
	}

	if !p.plan.systemCommand {
		if err := stageWindowsReadOnlyWorkspace(p.plan.commandDir, extensionDir); err != nil {
			p.closePipesLocked()
			return fmt.Errorf("stage Windows extension command bundle: %w", err)
		}
	}
	if p.plan.sourceWorkspace != "" {
		if err := stageWindowsReadOnlyWorkspace(p.plan.sourceWorkspace, workspaceDir); err != nil {
			p.closePipesLocked()
			return fmt.Errorf("stage Windows extension working directory: %w", err)
		}
	}

	application := p.plan.sourceCommand
	if !p.plan.systemCommand {
		relativeCommand, err := filepath.Rel(p.plan.commandDir, p.plan.sourceCommand)
		if err != nil || relativeCommand == ".." || strings.HasPrefix(relativeCommand, ".."+string(filepath.Separator)) {
			p.closePipesLocked()
			return fmt.Errorf("extension command escaped staged command directory")
		}
		application = filepath.Join(extensionDir, relativeCommand)
	}
	args, err := remapWindowsExtensionArgs(p.spec.Args, p.plan, extensionDir, workspaceDir)
	if err != nil {
		p.closePipesLocked()
		return err
	}
	if extension := strings.ToLower(filepath.Ext(application)); extension == ".cmd" || extension == ".bat" {
		systemDir, err := windows.GetSystemDirectory()
		if err != nil {
			p.closePipesLocked()
			return fmt.Errorf("resolve Windows system directory: %w", err)
		}
		scriptCommand := windowsCommandLine(application, args)
		application = filepath.Join(systemDir, "cmd.exe")
		args = []string{"/d", "/s", "/c", scriptCommand}
	}

	workingDirectory := homeDir
	switch p.plan.workRoot {
	case "extension":
		workingDirectory = filepath.Join(extensionDir, p.plan.workRelative)
	case "workspace":
		workingDirectory = filepath.Join(workspaceDir, p.plan.workRelative)
	case "home":
		workingDirectory = homeDir
	default:
		p.closePipesLocked()
		return fmt.Errorf("invalid Windows extension working-directory plan")
	}
	info, err := os.Stat(workingDirectory)
	if err != nil || !info.IsDir() {
		p.closePipesLocked()
		return fmt.Errorf("Windows extension working directory is unavailable")
	}
	environment, err := windowsExtensionEnvironment(profileRoot, extensionDir, homeDir, tmpDir, p.plan.systemCommand, p.spec.Env)
	if err != nil {
		p.closePipesLocked()
		return err
	}

	job, err := createWindowsKillOnCloseJob()
	if err != nil {
		p.closePipesLocked()
		return err
	}
	processInfo, err := startWindowsExtensionAppContainerProcess(application, args, workingDirectory, environment, appSID, job, p.pipes)
	if err != nil {
		_ = windows.CloseHandle(job)
		p.closePipesLocked()
		return err
	}
	_ = windows.CloseHandle(processInfo.Thread)
	p.pipes.closeChildEnds()

	p.job = job
	p.process = processInfo.Process
	p.profileName = profileName
	p.profileRoot = profileRoot
	p.started = true
	cleanupProfile = false

	go func() {
		select {
		case <-p.ctx.Done():
			_ = p.Kill()
		case <-p.waitDone:
		}
	}()
	return nil
}

func (p *windowsExtensionProcess) Wait() error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return fmt.Errorf("process has not started")
	}
	p.mu.Unlock()
	p.waitOnce.Do(func() { go p.waitProcess() })
	<-p.waitDone
	p.mu.Lock()
	err := p.waitErr
	p.mu.Unlock()
	return err
}

func (p *windowsExtensionProcess) waitProcess() {
	p.mu.Lock()
	process, job := p.process, p.job
	profileName, profileRoot := p.profileName, p.profileRoot
	p.mu.Unlock()

	var resultErr error
	waitResult, waitErr := windows.WaitForSingleObject(process, windows.INFINITE)
	if waitErr != nil {
		resultErr = fmt.Errorf("wait for Windows extension process: %w", waitErr)
	} else if waitResult != windows.WAIT_OBJECT_0 {
		resultErr = fmt.Errorf("unexpected Windows extension wait result %#x", waitResult)
	}
	_ = windows.TerminateJobObject(job, 1)
	if resultErr == nil {
		var exitCode uint32
		if err := windows.GetExitCodeProcess(process, &exitCode); err != nil {
			resultErr = fmt.Errorf("query Windows extension exit code: %w", err)
		} else if exitCode != 0 {
			resultErr = &windowsExtensionExitError{code: exitCode}
		}
	}
	if ctxErr := p.ctx.Err(); ctxErr != nil {
		resultErr = ctxErr
	}

	p.mu.Lock()
	if p.pipes != nil && p.pipes.parentStdin != nil {
		_ = p.pipes.parentStdin.Close()
		p.pipes.parentStdin = nil
	}
	if p.process != 0 {
		_ = windows.CloseHandle(p.process)
		p.process = 0
	}
	if p.job != 0 {
		_ = windows.CloseHandle(p.job)
		p.job = 0
	}
	p.mu.Unlock()

	if cleanupErr := cleanupWindowsExtensionProfile(profileName, profileRoot); cleanupErr != nil {
		if resultErr == nil {
			resultErr = cleanupErr
		} else {
			resultErr = errors.Join(resultErr, cleanupErr)
		}
	}
	p.mu.Lock()
	p.waitErr = resultErr
	p.mu.Unlock()
	close(p.waitDone)
}

func (p *windowsExtensionProcess) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return fmt.Errorf("process has not started")
	}
	if p.job == 0 {
		return nil
	}
	if err := windows.TerminateJobObject(p.job, 1); err != nil {
		return fmt.Errorf("terminate Windows extension process tree: %w", err)
	}
	return nil
}

func (p *windowsExtensionProcess) closePipesLocked() {
	if p.pipes != nil {
		p.pipes.closeAll()
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

func cleanupWindowsExtensionProfile(profileName, profileRoot string) error {
	var lastErr error
	for attempt := 0; attempt < windowsExtensionCleanupAttempts; attempt++ {
		if profileRoot != "" {
			if err := os.RemoveAll(profileRoot); err != nil {
				lastErr = fmt.Errorf("remove Windows extension profile data: %w", err)
				if attempt+1 < windowsExtensionCleanupAttempts {
					time.Sleep(50 * time.Millisecond)
				}
				continue
			}
		}
		if err := deleteWindowsAppContainerProfile(profileName); err != nil {
			lastErr = err
			if attempt+1 < windowsExtensionCleanupAttempts {
				time.Sleep(50 * time.Millisecond)
			}
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("Windows extension profile cleanup did not complete")
	}
	return lastErr
}

type windowsExtensionExitError struct{ code uint32 }

func (e *windowsExtensionExitError) Error() string {
	return fmt.Sprintf("Windows extension process exited with code %d", e.code)
}
func (e *windowsExtensionExitError) ExitCode() int { return int(e.code) }

var _ CommandProcess = (*windowsExtensionProcess)(nil)
