//go:build windows

package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type windowsExtensionLaunchPlan struct {
	sourceCommand   string
	commandDir      string
	systemCommand   bool
	sourceWorkspace string
	workRoot        string
	workRelative    string
}

func platformExtensionCommandContext(ctx context.Context, spec ProcessSpec, mode ExtensionSandboxMode) (CommandProcess, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := windowsExtensionAppContainerAvailable(); err != nil {
		if mode == ExtensionSandboxRequired {
			return nil, fmt.Errorf("persistent extension sandbox is required: %w", err)
		}
		return (HostCommandRunner{}).CommandContext(ctx, spec)
	}
	if err := validateExtensionEnvironment(spec); err != nil {
		return nil, err
	}
	plan, err := planWindowsExtensionLaunch(spec)
	if err != nil {
		return nil, err
	}
	return newWindowsExtensionProcess(ctx, spec, plan), nil
}

func windowsExtensionAppContainerAvailable() error {
	major, _, _ := windows.RtlGetNtVersionNumbers()
	if major < 10 {
		return fmt.Errorf("Windows AppContainer extension sandbox requires Windows 10 or newer")
	}
	for name, proc := range map[string]*windows.LazyProc{
		"CreateAppContainerProfile":                 windowsCreateAppContainerProfileProc,
		"DeriveAppContainerSidFromAppContainerName": windowsDeriveAppContainerSIDProc,
		"DeleteAppContainerProfile":                 windowsDeleteAppContainerProfileProc,
		"GetAppContainerFolderPath":                 windowsGetAppContainerFolderPathProc,
	} {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("Windows AppContainer API %s is unavailable: %w", name, err)
		}
	}
	return nil
}

func planWindowsExtensionLaunch(spec ProcessSpec) (windowsExtensionLaunchPlan, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("process command is required")
	}
	if strings.ContainsRune(command, '\x00') {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("process command contains NUL")
	}
	for _, arg := range spec.Args {
		if strings.ContainsRune(arg, '\x00') {
			return windowsExtensionLaunchPlan{}, fmt.Errorf("process argument contains NUL")
		}
	}

	resolved := command
	var err error
	if !filepath.IsAbs(resolved) && filepath.VolumeName(resolved) == "" {
		resolved, err = exec.LookPath(resolved)
		if err != nil {
			return windowsExtensionLaunchPlan{}, fmt.Errorf("resolve extension command %q: %w", command, err)
		}
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("absolute extension command: %w", err)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("resolve extension command: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("inspect extension command: %w", err)
	}
	if !info.Mode().IsRegular() {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("extension command must be a regular file")
	}

	extension := strings.ToLower(filepath.Ext(resolved))
	if extension != ".exe" && extension != ".com" && extension != ".cmd" && extension != ".bat" {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("Windows confined extension command %q must be an executable or cmd/bat script", resolved)
	}
	commandDir, err := windowsCanonicalDirectory(filepath.Dir(resolved))
	if err != nil {
		return windowsExtensionLaunchPlan{}, fmt.Errorf("resolve extension command directory: %w", err)
	}
	plan := windowsExtensionLaunchPlan{
		sourceCommand: resolved,
		commandDir:    commandDir,
		systemCommand: windowsExtensionSystemExecutable(resolved),
		workRoot:      "extension",
		workRelative:  ".",
	}
	if plan.systemCommand {
		plan.workRoot = "home"
	}

	if strings.TrimSpace(spec.Dir) != "" {
		working, err := windowsCanonicalDirectory(spec.Dir)
		if err != nil {
			return windowsExtensionLaunchPlan{}, fmt.Errorf("extension working directory: %w", err)
		}
		if !plan.systemCommand && windowsPathWithin(commandDir, working) {
			relative, err := filepath.Rel(commandDir, working)
			if err != nil {
				return windowsExtensionLaunchPlan{}, err
			}
			plan.workRoot = "extension"
			plan.workRelative = relative
		} else {
			plan.sourceWorkspace = working
			plan.workRoot = "workspace"
			plan.workRelative = "."
		}
	} else if inferred, err := inferWindowsExtensionWorkspace(spec.Args, commandDir, plan.systemCommand); err != nil {
		return windowsExtensionLaunchPlan{}, err
	} else if inferred != "" {
		plan.sourceWorkspace = inferred
		plan.workRoot = "workspace"
		plan.workRelative = "."
	}
	return plan, nil
}

func windowsExtensionSystemExecutable(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".exe" && extension != ".com" {
		return false
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(filepath.Dir(path)), filepath.Clean(systemDir))
}

func inferWindowsExtensionWorkspace(args []string, commandDir string, systemCommand bool) (string, error) {
	var root string
	for _, arg := range args {
		if !filepath.IsAbs(arg) && filepath.VolumeName(arg) == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(arg)
		if err != nil {
			continue
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return "", err
		}
		if !systemCommand && windowsPathWithin(commandDir, resolved) {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			continue
		}
		candidate := resolved
		if !info.IsDir() {
			candidate = filepath.Dir(resolved)
		}
		candidate, err = windowsCanonicalDirectory(candidate)
		if err != nil {
			return "", err
		}
		if root == "" {
			root = candidate
			continue
		}
		if !windowsPathWithin(root, candidate) && !windowsPathWithin(candidate, root) {
			return "", fmt.Errorf("Windows confined extension arguments reference multiple unrelated host roots")
		}
		if windowsPathWithin(candidate, root) {
			root = candidate
		}
	}
	return root, nil
}

func remapWindowsExtensionArgs(args []string, plan windowsExtensionLaunchPlan, extensionDir, workspaceDir string) ([]string, error) {
	out := append([]string(nil), args...)
	for index, arg := range out {
		if !filepath.IsAbs(arg) && filepath.VolumeName(arg) == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(arg)
		if err != nil {
			return nil, fmt.Errorf("Windows confined extension cannot expose absolute host argument %q", arg)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return nil, err
		}
		if !plan.systemCommand && windowsPathWithin(plan.commandDir, resolved) {
			relative, err := filepath.Rel(plan.commandDir, resolved)
			if err != nil {
				return nil, err
			}
			out[index] = filepath.Join(extensionDir, relative)
			continue
		}
		if plan.sourceWorkspace != "" && windowsPathWithin(plan.sourceWorkspace, resolved) {
			relative, err := filepath.Rel(plan.sourceWorkspace, resolved)
			if err != nil {
				return nil, err
			}
			out[index] = filepath.Join(workspaceDir, relative)
			continue
		}
		if plan.systemCommand && windowsExtensionSystemExecutable(resolved) {
			out[index] = resolved
			continue
		}
		return nil, fmt.Errorf("Windows confined extension rejects absolute host argument outside staged roots: %q", arg)
	}
	return out, nil
}

func windowsExtensionEnvironment(profileRoot, extensionDir, homeDir, tmpDir string, systemCommand bool, explicit map[string]string) ([]uint16, error) {
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows system directory: %w", err)
	}
	windowsDir, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve Windows directory: %w", err)
	}
	pathValue := systemDir
	if !systemCommand {
		pathValue = extensionDir + string(os.PathListSeparator) + systemDir
	}
	values := map[string]string{
		"SystemRoot":   windowsDir,
		"WINDIR":       windowsDir,
		"PATH":         pathValue,
		"PATHEXT":      ".COM;.EXE;.BAT;.CMD",
		"COMSPEC":      filepath.Join(systemDir, "cmd.exe"),
		"HOME":         homeDir,
		"USERPROFILE":  homeDir,
		"APPDATA":      homeDir,
		"LOCALAPPDATA": profileRoot,
		"TEMP":         tmpDir,
		"TMP":          tmpDir,
	}
	reserved := make(map[string]struct{}, len(values))
	for key := range values {
		reserved[strings.ToUpper(key)] = struct{}{}
	}
	for key, value := range explicit {
		if err := validateEnvironmentEntry(key, value); err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(key)
		if _, exists := reserved[strings.ToUpper(trimmed)]; exists {
			return nil, fmt.Errorf("extension environment key %q is runtime-reserved on Windows", key)
		}
		values[trimmed] = value
	}
	return windowsEnvironmentBlock(values)
}
