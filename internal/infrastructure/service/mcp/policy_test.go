package mcp

import (
	"testing"

	"buatpostingan/internal/config"
)

func TestLooksMutating(t *testing.T) {
	if !LooksMutating("delete_file", "removes a file") {
		t.Fatal("expected mutating")
	}
	if LooksMutating("echo", "Echo a message back (read-only)") {
		t.Fatal("echo should not look mutating")
	}
	if !LooksMutating("run_shell", "") {
		t.Fatal("run_shell should look mutating")
	}
}

func TestCheckCallAllowed(t *testing.T) {
	srv := config.MCPServer{ID: "s", AllowMutations: false}
	if err := CheckCallAllowed(srv, "echo", "read only"); err != nil {
		t.Fatalf("echo: %v", err)
	}
	if err := CheckCallAllowed(srv, "delete_item", ""); err == nil || err.Error() != "mutation_denied" {
		t.Fatalf("want mutation_denied, got %v", err)
	}

	srv.DenyTools = []string{"echo"}
	if err := CheckCallAllowed(srv, "echo", ""); err == nil || err.Error() != "tool_denied" {
		t.Fatalf("want tool_denied, got %v", err)
	}

	srv = config.MCPServer{ID: "s", AllowTools: []string{"echo"}, AllowMutations: false}
	if err := CheckCallAllowed(srv, "other", ""); err == nil || err.Error() != "tool_not_allowed" {
		t.Fatalf("want tool_not_allowed, got %v", err)
	}
	if err := CheckCallAllowed(srv, "echo", ""); err != nil {
		t.Fatalf("echo allowlisted: %v", err)
	}

	// Mutations require allow_mutations AND non-empty allow_tools hit.
	srv = config.MCPServer{ID: "s", AllowMutations: true, AllowTools: []string{"delete_item"}}
	if err := CheckCallAllowed(srv, "delete_item", ""); err != nil {
		t.Fatalf("opt-in mutation: %v", err)
	}
	srv = config.MCPServer{ID: "s", AllowMutations: true}
	if err := CheckCallAllowed(srv, "delete_item", ""); err == nil || err.Error() != "mutation_denied" {
		t.Fatalf("broad mutation opt-in denied, got %v", err)
	}
}

func TestNamespaceParse(t *testing.T) {
	ns := Namespace("echo", "greet")
	if ns != "mcp__echo__greet" {
		t.Fatalf("ns=%q", ns)
	}
	s, tool, ok := ParseNamespaced(ns)
	if !ok || s != "echo" || tool != "greet" {
		t.Fatalf("parse: %q %q %v", s, tool, ok)
	}
	if _, _, ok := ParseNamespaced("echo"); ok {
		t.Fatal("plain name should not parse")
	}
}
