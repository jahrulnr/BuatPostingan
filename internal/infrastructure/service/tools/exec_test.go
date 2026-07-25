package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellExecBasicCommand(t *testing.T) {
	e := &shellExec{}
	res, err := e.run(context.Background(), map[string]any{
		"command": "echo hello",
		"timeout": 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res["ok"].(bool) {
		t.Fatalf("expected ok, got: %+v", res)
	}
	data := res["data"].(map[string]any)
	stdout := data["stdout"].(string)
	if !strings.Contains(stdout, "hello") {
		t.Fatalf("expected stdout to contain 'hello', got: %q", stdout)
	}
	if data["exit_code"].(int) != 0 {
		t.Fatalf("expected exit code 0, got: %d", data["exit_code"])
	}
}

func TestShellExecNonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exit command differs on Windows")
	}
	e := &shellExec{}
	res, err := e.run(context.Background(), map[string]any{
		"command": "exit 7",
		"timeout": 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["ok"].(bool) {
		t.Fatalf("expected failure for non-zero exit")
	}
	data := res["data"].(map[string]any)
	if data["exit_code"].(int) != 7 {
		t.Fatalf("expected exit code 7, got: %d", data["exit_code"])
	}
}

func TestShellExecExplicitShellPath(t *testing.T) {
	e := &shellExec{}
	var shell string
	if runtime.GOOS == "windows" {
		shell = "cmd"
	} else {
		shell = "/bin/sh"
	}
	res, err := e.run(context.Background(), map[string]any{
		"command": "echo explicit",
		"shell":   shell,
		"timeout": 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res["ok"].(bool) {
		t.Fatalf("expected ok, got: %+v", res)
	}
	data := res["data"].(map[string]any)
	if !strings.Contains(data["stdout"].(string), "explicit") {
		t.Fatalf("expected stdout to contain 'explicit', got: %q", data["stdout"])
	}
}

func TestShellExecCwd(t *testing.T) {
	tmp := t.TempDir()
	e := &shellExec{}
	if runtime.GOOS == "windows" {
		res, err := e.run(context.Background(), map[string]any{
			"command": "cd",
			"cwd":     tmp,
			"timeout": 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res["ok"].(bool) {
			t.Fatalf("expected ok, got: %+v", res)
		}
		data := res["data"].(map[string]any)
		got := strings.TrimSpace(data["stdout"].(string))
		want := strings.ToLower(tmp)
		if !strings.EqualFold(got, want) && !strings.Contains(strings.ToLower(got), want) {
			t.Fatalf("expected cwd %q, got %q", want, got)
		}
	} else {
		res, err := e.run(context.Background(), map[string]any{
			"command": "pwd",
			"cwd":     tmp,
			"timeout": 5,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res["ok"].(bool) {
			t.Fatalf("expected ok, got: %+v", res)
		}
		data := res["data"].(map[string]any)
		got := strings.TrimSpace(data["stdout"].(string))
		want, _ := filepath.EvalSymlinks(tmp)
		if want == "" {
			want = tmp
		}
		if !strings.EqualFold(got, want) {
			t.Fatalf("expected cwd %q, got %q", want, got)
		}
	}
}

func TestShellExecTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep syntax differs on Windows")
	}
	e := &shellExec{}
	res, err := e.run(context.Background(), map[string]any{
		"command": "sleep 10",
		"timeout": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["ok"].(bool) {
		t.Fatalf("expected timeout failure")
	}
	errObj := res["error"].(map[string]any)
	if errObj["code"].(string) != "timeout" {
		t.Fatalf("expected timeout code, got: %q", errObj["code"])
	}
}

func TestShellExecValidation(t *testing.T) {
	e := &shellExec{}
	res, err := e.run(context.Background(), map[string]any{
		"command": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res["ok"].(bool) {
		t.Fatalf("expected validation failure")
	}
	errObj := res["error"].(map[string]any)
	if errObj["code"].(string) != "validation" {
		t.Fatalf("expected validation code, got: %q", errObj["code"])
	}
}

func TestDetectShellType(t *testing.T) {
	cases := []struct {
		path string
		want shellType
	}{
		{"/bin/sh", shellSh},
		{"bash", shellBash},
		{"/usr/bin/zsh", shellZsh},
		{"/bin/ash", shellAsh},
		{"pwsh", shellPowerShell},
		{"powershell.exe", shellPowerShell},
		{"cmd", shellCmd},
		{"fish", shellUnknown},
	}
	for _, c := range cases {
		got := detectShellType(c.path)
		if got != c.want {
			t.Errorf("detectShellType(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestResolveShellFallback(t *testing.T) {
	e := &shellExec{}
	if runtime.GOOS == "windows" {
		shell := e.resolveShell("")
		if shell.shellType != shellPowerShell && shell.shellType != shellCmd {
			t.Fatalf("expected windows shell, got %v", shell.shellType)
		}
		return
	}
	shell := e.resolveShell("bash")
	if shell.shellType != shellBash {
		t.Fatalf("expected bash, got %v", shell.shellType)
	}
}

func TestResolveCwdWorkspace(t *testing.T) {
	tmp := t.TempDir()
	e := &shellExec{}
	ctx := WithWorkspace(context.Background(), tmp)
	cwd, err := e.resolveCwd(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(cwd, tmp) {
		t.Fatalf("expected cwd %q, got %q", tmp, cwd)
	}
}

func TestResolveCwdRelative(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	e := &shellExec{}
	ctx := WithWorkspace(context.Background(), tmp)
	cwd, err := e.resolveCwd(ctx, "sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(cwd, sub) {
		t.Fatalf("expected cwd %q, got %q", sub, cwd)
	}
}
