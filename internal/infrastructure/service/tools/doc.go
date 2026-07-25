// Package tools implements ToolRegistry for allowlisted reader tools:
// search_docs, list_dir, read_file, grep, read_attachment, read_image,
// web_search, web_fetch, list_skills, read_skill, list_mcp_tools, call_mcp_tool.
//
// Local development: list_dir / read_file / grep resolve against the real
// filesystem (absolute paths including "/" allowed). Options.FSRoot is only an
// optional relative-path base for tests — not a sandbox. Do not ship this
// unrestricted FS access to multi-tenant production without reintroducing a jail.
// search_docs still searches the docs/webchat corpus only.
// list_skills / read_skill are jailed to Options.SkillsRoot (BP_SKILLS_ROOT).
// MCP tools are progressive meta-tools over configured stdio servers; mutations
// are default-denied (see docs/architecture/mcp-support.md).
package tools

// Allowlisted tool names (reader / meta tools only; no mutation tools).
var Allowlist = []string{
	"search_docs",
	"list_dir",
	"read_file",
	"grep",
	"read_attachment",
	"read_image",
	"web_search",
	"web_fetch",
	"list_skills",
	"read_skill",
	"list_mcp_tools",
	"call_mcp_tool",
	"write_file",
	"edit_file",
	"delete_file",
}

const DefaultTopK = 5
