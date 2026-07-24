// Package tools implements ToolRegistry for allowlisted reader tools:
// search_docs, list_dir, read_file, grep, read_attachment, read_image,
// web_search, web_fetch.
//
// Local development: list_dir / read_file / grep resolve against the real
// filesystem (absolute paths including "/" allowed). Options.FSRoot is only an
// optional relative-path base for tests — not a sandbox. Do not ship this
// unrestricted FS access to multi-tenant production without reintroducing a jail.
// search_docs still searches the docs/webchat corpus only.
package tools

// Allowlisted tool names (no mutation / admin-route tools).
var Allowlist = []string{
	"search_docs",
	"list_dir",
	"read_file",
	"grep",
	"read_attachment",
	"read_image",
	"web_search",
	"web_fetch",
}

const DefaultTopK = 5
