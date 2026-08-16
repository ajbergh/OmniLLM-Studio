//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const linuxCgroupCleanupTimeout = 2 * time.Second

// linuxCgroupManager owns one explicitly delegated cgroup-v2 subtree for
// sandbox executions. Individual resource capabilities are enabled only when
// the operator-delegated root exposes the matching controller. Atomic process
// placement is probed independently so no untrusted process runs before its
// execution cgroup authority exists.
type linuxCgroupManager struct {
	root          string
	pidsEnabled   bool
	memoryEnabled bool
}

type linuxExecutionCgroup struct {
	path string
	file *os.File
}

func newLinuxCgroupManager(configuredRoot, probeExecutable string) (*linuxCgroupManager, error) {
	configuredRoot = strings.TrimSpace(configuredRoot)
	if configuredRoot == "" {
		return nil, nil
	}
	root, err := canonicalDirectory(configuredRoot)
	if err != nil {
		return nil, fmt.Errorf("sandbox cgroup root: %w", err)
	}
	controllers, err := readLinuxCgroupTokens(filepath.Join(root, "cgroup.controllers"))
	if err != nil {
		return nil, fmt.Errorf("read delegated cgroup controllers: %w", err)
	}

	manager := &linuxCgroupManager{root: root}
	if _, ok := controllers["pids"]; ok {
		if err := ensureLinuxCgroupController(root, "pids"); err != nil {
			return nil, err
		}
		manager.pidsEnabled = true
	}
	if _, ok := controllers["memory"]; ok {
		if err := ensureLinuxCgroupController(root, "memory"); err != nil {
			return nil, err
		}
		manager.memoryEnabled = true
	}
	if !manager.pidsEnabled && !manager.memoryEnabled {
		return nil, fmt.Errorf("sandbox cgroup root does not delegate pids or memory controllers")
	}

	probe, err := manager.createExecution(0, 0)
	if err != nil {
		return nil, fmt.Errorf("probe delegated cgroup controllers: %w", err)
	}
	probeErr := probeCloneIntoLinuxCgroup(probe, probeExecutable)
	cleanupErr := probe.cleanup()
	if probeErr != nil {
		return nil, fmt.Errorf("probe atomic cgroup process placement: %w", probeErr)
	}
	if cleanupErr != nil {
		return nil, fmt.Errorf("cleanup cgroup probe: %w", cleanupErr)
	}
	return manager, nil
}

func ensureLinuxCgroupController(root, controller string) error {
	subtreePath := filepath.Join(root, "cgroup.subtree_control")
	enabled, err := readLinuxCgroupTokens(subtreePath)
	if err != nil {
		return fmt.Errorf("read cgroup subtree controllers: %w", err)
	}
	if _, ok := enabled[controller]; ok {
		return nil
	}
	if err := os.WriteFile(subtreePath, []byte("+"+controller), 0); err != nil {
		return fmt.Errorf("enable delegated cgroup %s controller: %w", controller, err)
	}
	enabled, err = readLinuxCgroupTokens(subtreePath)
	if err != nil {
		return fmt.Errorf("verify cgroup subtree controllers: %w", err)
	}
	if _, ok := enabled[controller]; !ok {
		return fmt.Errorf("delegated cgroup %s controller did not become active", controller)
	}
	return nil
}

func readLinuxCgroupTokens(path string) (map[string]struct{}, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	for _, token := range strings.Fields(string(content)) {
		out[token] = struct{}{}
	}
	return out, nil
}

func (m *linuxCgroupManager) createExecution(maxProcesses int, memoryBytes int64) (*linuxExecutionCgroup, error) {
	if m == nil {
		return nil, fmt.Errorf("Linux cgroup manager is unavailable")
	}
	if maxProcesses < 0 {
		return nil, fmt.Errorf("Linux sandbox process limit cannot be negative")
	}
	if memoryBytes < 0 {
		return nil, fmt.Errorf("Linux sandbox memory limit cannot be negative")
	}
	if maxProcesses > 0 && !m.pidsEnabled {
		return nil, fmt.Errorf("Linux cgroup pids controller is unavailable")
	}
	if memoryBytes > 0 && !m.memoryEnabled {
		return nil, fmt.Errorf("Linux cgroup memory controller is unavailable")
	}

	dir, err := os.MkdirTemp(m.root, "omnillm-exec-")
	if err != nil {
		return nil, fmt.Errorf("create execution cgroup: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(dir)
		}
	}()

	if m.pidsEnabled {
		limit := "max"
		if maxProcesses > 0 {
			limit = strconv.Itoa(maxProcesses)
		}
		if err := os.WriteFile(filepath.Join(dir, "pids.max"), []byte(limit), 0); err != nil {
			return nil, fmt.Errorf("configure execution pids.max: %w", err)
		}
	}
	if m.memoryEnabled {
		limit := "max"
		if memoryBytes > 0 {
			limit = strconv.FormatInt(memoryBytes, 10)
		}
		if err := os.WriteFile(filepath.Join(dir, "memory.max"), []byte(limit), 0); err != nil {
			return nil, fmt.Errorf("configure execution memory.max: %w", err)
		}
	}

	file, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("open execution cgroup: %w", err)
	}
	cleanup = false
	return &linuxExecutionCgroup{path: dir, file: file}, nil
}

func (c *linuxExecutionCgroup) memoryEvents() (map[string]uint64, error) {
	if c == nil || strings.TrimSpace(c.path) == "" {
		return nil, fmt.Errorf("Linux execution cgroup is unavailable")
	}
	content, err := os.ReadFile(filepath.Join(c.path, "memory.events"))
	if err != nil {
		return nil, fmt.Errorf("read execution memory.events: %w", err)
	}
	events := make(map[string]uint64)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("parse execution memory.events line %q", line)
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse execution memory.events %s: %w", fields[0], err)
		}
		events[fields[0]] = value
	}
	return events, nil
}

func (c *linuxExecutionCgroup) attach(cmd *exec.Cmd) error {
	if c == nil || c.file == nil {
		return fmt.Errorf("Linux execution cgroup is unavailable")
	}
	if cmd == nil {
		return fmt.Errorf("sandbox command is required for cgroup attachment")
	}
	if cmd.SysProcAttr != nil {
		return fmt.Errorf("sandbox command already has Linux process attributes")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    int(c.file.Fd()),
	}
	return nil
}

func probeCloneIntoLinuxCgroup(cgroup *linuxExecutionCgroup, executable string) error {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return fmt.Errorf("cgroup process-placement probe executable is required")
	}
	cmd := exec.Command(executable, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cgroup.attach(cmd); err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (c *linuxExecutionCgroup) cleanup() error {
	if c == nil {
		return nil
	}
	if c.file != nil {
		if err := c.file.Close(); err != nil {
			return fmt.Errorf("close execution cgroup: %w", err)
		}
		c.file = nil
	}

	killPath := filepath.Join(c.path, "cgroup.kill")
	if err := os.WriteFile(killPath, []byte("1"), 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("kill execution cgroup: %w", err)
	}
	deadline := time.Now().Add(linuxCgroupCleanupTimeout)
	for {
		content, err := os.ReadFile(filepath.Join(c.path, "cgroup.procs"))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read execution cgroup processes: %w", err)
		}
		if err == nil && strings.TrimSpace(string(content)) == "" {
			break
		}
		if os.IsNotExist(err) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("execution cgroup remained populated during cleanup")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove execution cgroup: %w", err)
	}
	return nil
}
