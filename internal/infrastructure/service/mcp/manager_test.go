package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"buatpostingan/internal/config"
	"buatpostingan/internal/pkg/logging"
)

func TestManagerStdioEcho(t *testing.T) {
	bin := buildEchoBinary(t)
	mgr := NewManager(config.Config{
		MCPEnabled:           true,
		MCPConnectTimeoutSec: 20,
		MCPCallTimeoutSec:    20,
		MCPServers: []config.MCPServer{{
			ID:         "echo",
			Transport:  "stdio",
			Command:    bin,
			Enabled:    true,
			Trusted:    true,
			AllowTools: []string{"echo"},
		}},
	})
	defer func() { _ = mgr.Close() }()

	ctx := logging.WithTraceID(context.Background(), "tr_mcp_test")
	tools, errs := mgr.ListTools(ctx, "echo")
	if len(errs) > 0 {
		t.Fatalf("server errors: %+v", errs)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("tools=%+v", tools)
	}
	if tools[0].Namespaced != "mcp__echo__echo" {
		t.Fatalf("namespaced=%q", tools[0].Namespaced)
	}
	if !tools[0].Allowed {
		t.Fatal("echo should be allowed")
	}

	res, err := mgr.CallTool(ctx, "echo", "echo", map[string]any{"message": "ping"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res)
	}
	if res.RawText == "" && len(res.Content) == 0 {
		t.Fatalf("empty result: %+v", res)
	}
	if !res.Trusted {
		t.Fatal("trusted server should mark Trusted")
	}
}

func TestManagerIsolatesBadServer(t *testing.T) {
	bin := buildEchoBinary(t)
	mgr := NewManager(config.Config{
		MCPEnabled:           true,
		MCPConnectTimeoutSec: 5,
		MCPCallTimeoutSec:    5,
		MCPServers: []config.MCPServer{
			{
				ID: "bad", Transport: "stdio", Command: "/nonexistent/mcp-bin",
				Enabled: true,
			},
			{
				ID: "echo", Transport: "stdio", Command: bin,
				Enabled: true, AllowTools: []string{"echo"},
			},
		},
	})
	defer func() { _ = mgr.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tools, errs := mgr.ListTools(ctx, "")
	if errs["bad"] == "" {
		t.Fatalf("expected bad server error, got %+v", errs)
	}
	found := false
	for _, tl := range tools {
		if tl.Server == "echo" && tl.Name == "echo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("echo tools missing despite bad peer: tools=%+v errs=%+v", tools, errs)
	}
}

func TestManagerMissingBinarySurfacesError(t *testing.T) {
	mgr := NewManager(config.Config{
		MCPEnabled:           true,
		MCPConnectTimeoutSec: 2,
		MCPCallTimeoutSec:    2,
		MCPServers: []config.MCPServer{{
			ID: "echo", Transport: "stdio", Command: "./bin/mcp-echo-missing", Enabled: true,
		}},
	})
	defer func() { _ = mgr.Close() }()
	_, errs := mgr.ListTools(context.Background(), "")
	if errs["echo"] == "" {
		t.Fatalf("expected server_errors for missing binary, got %+v", errs)
	}
	if !strings.Contains(errs["echo"], "not found") {
		t.Fatalf("want not-found style error, got %q", errs["echo"])
	}
}

func buildEchoBinary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "mcp-echo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/mcp-echo")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build mcp-echo: %v\n%s", err, out)
	}
	return bin
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
