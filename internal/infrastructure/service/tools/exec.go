package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// shellExec runs a command in a shell with cross-platform terminal support.
// It mirrors codex shell detection and argument derivation for sh/bash/zsh/ash,
// PowerShell, and cmd.exe.
type shellExec struct{}

// shellType identifies a supported shell family.
type shellType int

const (
	shellUnknown shellType = iota
	shellSh
	shellBash
	shellZsh
	shellAsh
	shellPowerShell
	shellCmd
)

func (s shellType) name() string {
	switch s {
	case shellSh:
		return "sh"
	case shellBash:
		return "bash"
	case shellZsh:
		return "zsh"
	case shellAsh:
		return "ash"
	case shellPowerShell:
		return "powershell"
	case shellCmd:
		return "cmd"
	default:
		return "unknown"
	}
}

func (s shellType) arg() string {
	switch s {
	case shellSh, shellBash, shellZsh, shellAsh:
		return "-c"
	case shellPowerShell:
		return "-Command"
	case shellCmd:
		return "/c"
	default:
		return "-c"
	}
}

func (s shellType) extraArgs() []string {
	if s == shellPowerShell {
		return []string{"-NoProfile"}
	}
	return nil
}

type detectedShell struct {
	shellType shellType
	path      string
}

func (e *shellExec) run(ctx context.Context, args map[string]any) (map[string]any, error) {
	command := strings.TrimSpace(asString(args["command"]))
	if command == "" {
		return failMap("validation", "command is required"), nil
	}

	shell := e.resolveShell(asString(args["shell"]))
	cwd, err := e.resolveCwd(ctx, asString(args["cwd"]))
	if err != nil {
		return failMap("invalid_cwd", err.Error()), nil
	}

	timeoutSec := clamp(asInt(args["timeout"], 60), 1, 300)

	cmdArgs := append([]string{}, shell.path)
	cmdArgs = append(cmdArgs, shell.shellType.extraArgs()...)
	cmdArgs = append(cmdArgs, shell.shellType.arg(), command)

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()

	outStr := stdout.String()
	errStr := stderr.String()
	exitCode := 0

	if execCtx.Err() == context.DeadlineExceeded {
		return failMap("timeout", fmt.Sprintf("command timed out after %ds", timeoutSec)), nil
	}

	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			if ws, ok := ee.ProcessState.Sys().(syscall.WaitStatus); ok {
				exitCode = ws.ExitStatus()
			}
		} else {
			return failMap("exec_error", runErr.Error()), nil
		}
	} else if cmd.ProcessState != nil {
		if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			exitCode = ws.ExitStatus()
		}
	}

	const maxOutput = 100000
	if len(outStr) > maxOutput {
		outStr = outStr[:maxOutput] + "\n[truncated]"
	}
	if len(errStr) > maxOutput {
		errStr = errStr[:maxOutput] + "\n[truncated]"
	}

	ok := exitCode == 0
	data := map[string]any{
		"shell":     shell.shellType.name(),
		"shellPath": shell.path,
		"cwd":       cwd,
		"exit_code": exitCode,
		"stdout":    outStr,
		"stderr":    errStr,
	}

	if !ok {
		return map[string]any{
			"ok":    false,
			"tool":  "exec",
			"data":  data,
			"error": map[string]any{"code": "non_zero_exit", "message": fmt.Sprintf("exit code %d", exitCode)},
			"meta": map[string]any{
				"truncated":         len(outStr) >= maxOutput || len(errStr) >= maxOutput,
				"count":             0,
				"data_is_untrusted": true,
			},
		}, nil
	}

	return okMap(data, false), nil
}

// resolveShell detects the shell to use. Explicit shell path/name wins, then the
// user's default shell, then platform fallbacks.
func (e *shellExec) resolveShell(explicit string) detectedShell {
	if explicit != "" {
		t := detectShellType(explicit)
		if t == shellUnknown {
			t = shellFromExplicitName(explicit)
		}
		if t != shellUnknown {
			path := explicit
			if !filepath.IsAbs(path) {
				if found, err := exec.LookPath(filepath.Base(path)); err == nil {
					path = found
				}
			}
			if fileExists(path) {
				return detectedShell{shellType: t, path: path}
			}
			if found := findShellPath(t); found != "" {
				return detectedShell{shellType: t, path: found}
			}
		}
	}

	if shell := defaultUserShell(); shell.shellType != shellUnknown {
		return shell
	}
	if runtime.GOOS == "windows" {
		if p := findShellPath(shellCmd); p != "" {
			return detectedShell{shellType: shellCmd, path: p}
		}
	}
	for _, t := range []shellType{shellBash, shellZsh, shellSh, shellAsh} {
		if p := findShellPath(t); p != "" {
			return detectedShell{shellType: t, path: p}
		}
	}
	return detectedShell{shellType: shellSh, path: "/bin/sh"}
}

// resolveCwd returns the working directory. Explicit cwd wins, then workspace
// context, then process cwd. Relative cwd resolves from the workspace or process cwd.
func (e *shellExec) resolveCwd(ctx context.Context, explicit string) (string, error) {
	base := workspaceFrom(ctx)
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	cwd := strings.TrimSpace(explicit)
	if cwd == "" {
		return base, nil
	}

	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(base, cwd)
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("cwd is not a directory: %s", cwd)
	}
	return abs, nil
}

func detectShellType(path string) shellType {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".exe" {
		base = base[:len(base)-len(ext)]
	}
	switch base {
	case "sh":
		return shellSh
	case "bash":
		return shellBash
	case "zsh":
		return shellZsh
	case "ash":
		return shellAsh
	case "pwsh", "powershell":
		return shellPowerShell
	case "cmd":
		return shellCmd
	}
	return shellUnknown
}

func shellFromExplicitName(name string) shellType {
	n := strings.ToLower(strings.TrimSpace(filepath.Base(name)))
	if strings.Contains(n, "powershell") || n == "pwsh" {
		return shellPowerShell
	}
	if n == "cmd" {
		return shellCmd
	}
	if n == "bash" {
		return shellBash
	}
	if n == "zsh" {
		return shellZsh
	}
	if n == "ash" {
		return shellAsh
	}
	if n == "sh" {
		return shellSh
	}
	return shellUnknown
}

// defaultUserShell returns the user's login shell on Unix (from passwd/SHELL env)
// or attempts PowerShell on Windows.
func defaultUserShell() detectedShell {
	if runtime.GOOS == "windows" {
		if p := findShellPath(shellPowerShell); p != "" {
			return detectedShell{shellType: shellPowerShell, path: p}
		}
		if p := findShellPath(shellCmd); p != "" {
			return detectedShell{shellType: shellCmd, path: p}
		}
		return detectedShell{shellType: shellCmd, path: "cmd.exe"}
	}

	if shellEnv := os.Getenv("SHELL"); shellEnv != "" {
		t := detectShellType(shellEnv)
		if t != shellUnknown && fileExists(shellEnv) {
			return detectedShell{shellType: t, path: shellEnv}
		}
	}

	if u, err := user.Current(); err == nil && u.Username != "" {
		t := detectShellType(u.HomeDir)
		_ = t
	}

	// Try getlogin shell via cgo-free user.Lookup.
	if u, err := user.LookupId(fmt.Sprintf("%d", os.Getuid())); err == nil {
		if shell, ok := userShellFromPasswd(u.Username); ok {
			t := detectShellType(shell)
			if t != shellUnknown && fileExists(shell) {
				return detectedShell{shellType: t, path: shell}
			}
		}
	}

	for _, t := range []shellType{shellBash, shellZsh, shellSh, shellAsh} {
		if p := findShellPath(t); p != "" {
			return detectedShell{shellType: t, path: p}
		}
	}
	return detectedShell{shellType: shellSh, path: "/bin/sh"}
}

func findShellPath(t shellType) string {
	candidates := shellFallbackPaths(t)
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	name := t.name()
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func shellFallbackPaths(t shellType) []string {
	switch t {
	case shellBash:
		return []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash"}
	case shellZsh:
		return []string{"/bin/zsh", "/usr/bin/zsh", "/usr/local/bin/zsh"}
	case shellSh:
		return []string{"/bin/sh", "/usr/bin/sh"}
	case shellAsh:
		return []string{"/bin/ash", "/usr/bin/ash", "/bin/busybox"}
	case shellPowerShell:
		if runtime.GOOS == "windows" {
			return []string{
				`C:\Program Files\PowerShell\7\pwsh.exe`,
				`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
			}
		}
		return []string{"/usr/local/bin/pwsh", "/usr/bin/pwsh"}
	case shellCmd:
		if runtime.GOOS == "windows" {
			return []string{`C:\Windows\System32\cmd.exe`}
		}
		return nil
	}
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// userShellFromPasswd reads /etc/passwd and returns the shell for the named user.
// Kept lightweight; user.Current does not expose the shell on all platforms.
func userShellFromPasswd(username string) (string, bool) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 7 && parts[0] == username {
			return parts[6], true
		}
	}
	return "", false
}
