package git

import (
	"slices"
	"testing"
)

func TestGenerateCommitMessage(t *testing.T) {
	tests := []struct {
		name string
		cs   *ChangeSet
		// want lists every accepted message; group order follows map iteration.
		want []string
	}{
		{
			name: "SingleAdd",
			cs:   &ChangeSet{Added: []EntityChange{{Type: "ticket", ID: "TKT-001", IsNew: true}}},
			want: []string{"Add ticket TKT-001"},
		},
		{
			name: "MultipleAdds",
			cs: &ChangeSet{Added: []EntityChange{
				{Type: "ticket", ID: "TKT-001", IsNew: true},
				{Type: "ticket", ID: "TKT-002", IsNew: true},
			}},
			want: []string{"Add ticket TKT-001, TKT-002", "Add ticket TKT-002, TKT-001"},
		},
		{
			name: "ManyAdds",
			cs: &ChangeSet{Added: []EntityChange{
				{Type: "ticket", ID: "TKT-001", IsNew: true},
				{Type: "ticket", ID: "TKT-002", IsNew: true},
				{Type: "ticket", ID: "TKT-003", IsNew: true},
				{Type: "ticket", ID: "TKT-004", IsNew: true},
			}},
			want: []string{"Add 4 tickets"},
		},
		{
			name: "SingleModify",
			cs: &ChangeSet{Modified: []EntityChange{
				{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status", "priority"}},
			}},
			want: []string{"TKT-001: update status, priority"},
		},
		{
			name: "SingleModifyBodyOnly",
			cs:   &ChangeSet{Modified: []EntityChange{{Type: "ticket", ID: "TKT-001", BodyChanged: true}}},
			want: []string{"TKT-001: update description"},
		},
		{
			name: "SingleModifyUnknown",
			cs:   &ChangeSet{Modified: []EntityChange{{Type: "ticket", ID: "TKT-001"}}},
			want: []string{"TKT-001: update"},
		},
		{
			name: "MultipleModify",
			cs: &ChangeSet{Modified: []EntityChange{
				{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status"}},
				{Type: "ticket", ID: "TKT-002", PropsChanged: []string{"priority"}},
			}},
			want: []string{"Update ticket TKT-001, TKT-002", "Update ticket TKT-002, TKT-001"},
		},
		{
			name: "SingleDelete",
			cs:   &ChangeSet{Deleted: []EntityRef{{Type: "ticket", ID: "TKT-001"}}},
			want: []string{"Remove ticket TKT-001"},
		},
		{
			name: "MultipleDeletes",
			cs: &ChangeSet{Deleted: []EntityRef{
				{Type: "ticket", ID: "TKT-001"},
				{Type: "ticket", ID: "TKT-002"},
			}},
			want: []string{"Remove 2 entities"},
		},
		{
			name: "SingleRelation",
			cs: &ChangeSet{Relations: []RelationChange{
				{From: "TKT-001", RelType: "blocks", To: "TKT-002", IsNew: true},
			}},
			want: []string{"Link TKT-001 -> TKT-002"},
		},
		{
			name: "MultipleRelations",
			cs: &ChangeSet{Relations: []RelationChange{
				{From: "TKT-001", RelType: "blocks", To: "TKT-002", IsNew: true},
				{From: "TKT-001", RelType: "blocks", To: "TKT-003", IsNew: true},
			}},
			want: []string{"Add 2 relations"},
		},
		{
			name: "Combined",
			cs: &ChangeSet{
				Added: []EntityChange{{Type: "ticket", ID: "TKT-003", IsNew: true}},
				Modified: []EntityChange{
					{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status"}},
				},
			},
			want: []string{"Add ticket TKT-003; TKT-001: update status"},
		},
		{
			name: "Empty",
			cs:   &ChangeSet{},
			want: []string{"Update entities"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := tc.cs.GenerateCommitMessage()
			if !slices.Contains(tc.want, msg) {
				t.Errorf("expected one of %q, got %q", tc.want, msg)
			}
		})
	}
}

func TestGenerateCommitMessage_Truncation(t *testing.T) {
	cs := &ChangeSet{
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001", PropsChanged: []string{
				"status", "priority", "assignee", "reporter", "due_date",
				"estimated_hours", "actual_hours", "description", "title",
			}},
		},
	}
	msg := cs.GenerateCommitMessage()
	if len(msg) > maxCommitMsgLen {
		t.Errorf("message should be truncated to %d chars, got %d: %q", maxCommitMsgLen, len(msg), msg)
	}
	if len(msg) > 3 && msg[len(msg)-3:] != "..." {
		t.Errorf("truncated message should end with '...', got %q", msg)
	}
}

func TestGroupByType(t *testing.T) {
	changes := []EntityChange{
		{Type: "ticket", ID: "TKT-001"},
		{Type: "ticket", ID: "TKT-002"},
		{Type: "category", ID: "CAT-001"},
	}
	result := groupByType(changes)
	if len(result["ticket"]) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(result["ticket"]))
	}
	if len(result["category"]) != 1 {
		t.Errorf("expected 1 category, got %d", len(result["category"]))
	}
}

func TestExtractIDs(t *testing.T) {
	changes := []EntityChange{
		{Type: "ticket", ID: "TKT-001"},
		{Type: "ticket", ID: "TKT-002"},
	}
	ids := extractIDs(changes)
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
}
