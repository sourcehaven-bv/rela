package mcp

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// knownToolNames is every tool this package can register, whichever features a
// server has on. [toolFilter.check] judges names against it, not against what
// one server registered: a deny entry for a feature that is off filters
// nothing, and must not fail the build when a Configure save turns comments
// off or a tenant has no actions. TestKnownToolNames_MatchesRegistrations pins it
// to the newTool calls.
var knownToolNames = map[string]bool{
	"accept_comment_suggestion": true, "add_comment": true, "add_to_pile": true,
	"analyze": true, "attach_file": true, "claim_entity": true,
	"complete_flow_step": true, "convert_entity": true, "create_channel": true,
	"create_entity": true, "create_relation": true, "delete_attachment": true,
	"delete_comment": true, "delete_entity": true, "delete_relation": true,
	"edit_message": true, "find_path": true, "list_actions": true,
	"list_attachments": true, "list_channels": true, "list_comments": true,
	"list_conversions": true, "list_entities": true, "list_flow_tasks": true,
	"list_inbox": true, "list_leases": true, "list_piles": true,
	"list_relations": true, "list_worlds": true, "lua_eval": true,
	"lua_run": true, "mark_read": true, "post_message": true,
	"post_messages": true, "read_attachment": true, "read_channel": true,
	"read_message_file": true, "read_session": true, "release_lease": true,
	"remove_from_pile": true, "remove_message": true, "rename_entity": true,
	"renew_lease": true, "run_action": true, "schema": true,
	"search_entities": true, "show_entity": true, "show_flow_run": true,
	"show_pile": true, "trace": true, "update_comment": true,
	"update_entity": true, "watch": true,
}

// toolFilter decides which tools a server registers. The remote endpoint
// takes it from the operator's `remote_mcp.tools` in .rela/config.yaml; the
// stdio server has none.
//
// A filtered-out tool is not registered at all, so it is missing from
// `tools/list` and a call to it is an unknown-tool error. The watch tool
// cannot replay it either, because registerTools drops it from the replay
// set. Resources and prompts are not filtered; see [WithToolFilter].
type toolFilter struct {
	// allow, when non-empty, is the only set of tools registered.
	allow map[string]bool
	// deny removes tools. It wins over allow.
	deny map[string]bool
}

// WithToolFilter limits the tools the server registers. With a non-empty
// allow, only those tools are registered; deny then removes tools from what
// is left. A name in either list that is not a rela MCP tool makes
// [NewServer] fail, so a misspelled entry cannot leave a tool on that the
// operator meant to turn off. A real tool whose feature is off is accepted
// and filters nothing.
//
// Only the remote wiring passes it. It narrows the tools, not read access:
// the `rela://entity/...` resources and the prompts still read whatever the
// caller's ACL grants, so the ACL stays the read boundary.
func WithToolFilter(allow, deny []string) Option {
	return func(s *Server) {
		s.tools = &toolFilter{allow: nameSet(allow), deny: nameSet(deny)}
	}
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[strings.TrimSpace(n)] = true
	}
	return set
}

// admits reports whether the filter lets the tool name register.
// Nil: no filter, every tool registers.
func (f *toolFilter) admits(name string) bool {
	if f == nil {
		return true
	}
	if f.deny[name] {
		return false
	}
	return len(f.allow) == 0 || f.allow[name]
}

// check refuses a name in allow or deny that is not in [knownToolNames].
// Nil: nothing to check.
func (f *toolFilter) check() error {
	if f == nil {
		return nil
	}
	var unknown []string
	for _, set := range []map[string]bool{f.allow, f.deny} {
		for name := range set {
			if !knownToolNames[name] && !slices.Contains(unknown, name) {
				unknown = append(unknown, name)
			}
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	return fmt.Errorf("%w: %s; the tools are: %s", errUnknownFilteredTool,
		strings.Join(unknown, ", "), strings.Join(slices.Sorted(maps.Keys(knownToolNames)), ", "))
}

// errUnknownFilteredTool is the error [NewServer] wraps when a tool filter
// names a tool rela does not have.
var errUnknownFilteredTool = errors.New("mcp: tool filter names tools rela does not have")
