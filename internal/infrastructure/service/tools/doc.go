// Package tools implements ToolRegistry for allowlisted reader tools and
// strictly jailed static-page lifecycle tools:
// docs_search, docs_read, docs_list, list_dir, read_file, grep, read_attachment,
// read_image, web_search, web_fetch, list_skills, read_skill, list_mcp_tools, call_mcp_tool.
//
// Local development: list_dir / read_file / grep resolve against the real
// filesystem (absolute paths including "/" allowed). Options.FSRoot is only an
// optional relative-path base for tests — not a sandbox. Do not ship this
// unrestricted FS access to multi-tenant production without reintroducing a jail.
// docs_search / docs_read / docs_list operate on the resources/webchat/docs corpus only.
// list_skills / read_skill are jailed to Options.SkillsRoot (BP_SKILLS_ROOT).
// Page lifecycle is restricted to Options.PagesRoot and represents publication
// exclusively through symlinks under .published. MCP tools are progressive
// meta-tools over configured stdio servers; mutations remain default-denied.
package tools

// Allowlisted tool names (reader / meta / safe host tools; exec is a controlled
// host mutation subject to the process user's OS permissions; page tools are
// constrained to the PagesRoot jail).
var Allowlist = []string{
	"docs_search",
	"docs_read",
	"docs_list",
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
	"exec",
	"page_list",
	"page_search",
	"page_create",
	"page_edit",
	"page_read",
	"page_publish",
	"page_unpublish",
}

const DefaultTopK = 5
