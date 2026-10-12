package mcp

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// A delete or rename a channel type blocks is reported as a conflict that
// names the type; any other error passes through as its text.
func TestWriteErrorResult(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{"blocked", fmt.Errorf("delete: %w", &entitymanager.AnchorBlockedError{ChannelType: "incident-room"}),
			[]string{"conflict:", "incident-room", "on_entity_delete: block"}},
		{"other", errors.New("disk full"), []string{"disk full"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := writeErrorResult(tc.err)
			if !res.IsError {
				t.Fatal("want an error result")
			}
			text := res.Content[0].(*mcpgo.TextContent).Text
			for _, w := range tc.want {
				if !strings.Contains(text, w) {
					t.Errorf("result %q lacks %q", text, w)
				}
			}
		})
	}
}
