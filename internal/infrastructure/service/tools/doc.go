// Package tools implements ToolRegistry for allowlisted docs tools:
// search_docs, list_dir, read_file, grep.
package tools

// Allowlisted tool names (no mutation / admin-route tools).
var Allowlist = []string{"search_docs", "list_dir", "read_file", "grep"}

const DefaultTopK = 5
