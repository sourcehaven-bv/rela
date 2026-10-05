package entitymanager

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestRequireAuthorizedEdges(t *testing.T) {
	draft := &entity.Relation{From: "POL-1", FromFace: "draft", Type: "implements", To: "CTL-1"}
	published := &entity.Relation{From: "POL-1", FromFace: "published", Type: "implements", To: "CTL-1"}
	inbound := &entity.Relation{From: "CTL-2", Type: "covers", To: "POL-1"}
	for _, tc := range []struct {
		name    string
		deleted []*entity.Relation
		wantErr bool
	}{
		{"nothing deleted", nil, false},
		{"only authorized", []*entity.Relation{draft, inbound}, false},
		{"same triple on another tail", []*entity.Relation{published}, true},
		{"unauthorized inbound", []*entity.Relation{draft, {From: "CTL-3", Type: "covers", To: "POL-1"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := requireAuthorizedEdges(tc.deleted, []*entity.Relation{inbound}, []*entity.Relation{draft})
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
