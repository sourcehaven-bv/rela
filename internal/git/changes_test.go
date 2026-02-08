package git

import (
	"testing"
)

func TestGenerateCommitMessage_SingleAdd(t *testing.T) {
	cs := &ChangeSet{
		Added: []EntityChange{
			{Type: "ticket", ID: "TKT-001", IsNew: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Add ticket TKT-001" {
		t.Errorf("expected 'Add ticket TKT-001', got %q", msg)
	}
}

func TestGenerateCommitMessage_MultipleAdds(t *testing.T) {
	cs := &ChangeSet{
		Added: []EntityChange{
			{Type: "ticket", ID: "TKT-001", IsNew: true},
			{Type: "ticket", ID: "TKT-002", IsNew: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	// Order may vary due to map iteration
	if msg != "Add ticket TKT-001, TKT-002" && msg != "Add ticket TKT-002, TKT-001" {
		t.Errorf("expected 'Add ticket TKT-001, TKT-002' or similar, got %q", msg)
	}
}

func TestGenerateCommitMessage_ManyAdds(t *testing.T) {
	cs := &ChangeSet{
		Added: []EntityChange{
			{Type: "ticket", ID: "TKT-001", IsNew: true},
			{Type: "ticket", ID: "TKT-002", IsNew: true},
			{Type: "ticket", ID: "TKT-003", IsNew: true},
			{Type: "ticket", ID: "TKT-004", IsNew: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Add 4 tickets" {
		t.Errorf("expected 'Add 4 tickets', got %q", msg)
	}
}

func TestGenerateCommitMessage_SingleModify(t *testing.T) {
	cs := &ChangeSet{
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status", "priority"}},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "TKT-001: update status, priority" {
		t.Errorf("expected 'TKT-001: update status, priority', got %q", msg)
	}
}

func TestGenerateCommitMessage_SingleModifyBodyOnly(t *testing.T) {
	cs := &ChangeSet{
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001", BodyChanged: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "TKT-001: update description" {
		t.Errorf("expected 'TKT-001: update description', got %q", msg)
	}
}

func TestGenerateCommitMessage_SingleModifyUnknown(t *testing.T) {
	cs := &ChangeSet{
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001"},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "TKT-001: update" {
		t.Errorf("expected 'TKT-001: update', got %q", msg)
	}
}

func TestGenerateCommitMessage_MultipleModify(t *testing.T) {
	cs := &ChangeSet{
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status"}},
			{Type: "ticket", ID: "TKT-002", PropsChanged: []string{"priority"}},
		},
	}
	msg := cs.GenerateCommitMessage()
	// Order may vary
	if msg != "Update ticket TKT-001, TKT-002" && msg != "Update ticket TKT-002, TKT-001" {
		t.Errorf("expected 'Update ticket TKT-001, TKT-002' or similar, got %q", msg)
	}
}

func TestGenerateCommitMessage_SingleDelete(t *testing.T) {
	cs := &ChangeSet{
		Deleted: []EntityRef{
			{Type: "ticket", ID: "TKT-001"},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Remove ticket TKT-001" {
		t.Errorf("expected 'Remove ticket TKT-001', got %q", msg)
	}
}

func TestGenerateCommitMessage_MultipleDeletes(t *testing.T) {
	cs := &ChangeSet{
		Deleted: []EntityRef{
			{Type: "ticket", ID: "TKT-001"},
			{Type: "ticket", ID: "TKT-002"},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Remove 2 entities" {
		t.Errorf("expected 'Remove 2 entities', got %q", msg)
	}
}

func TestGenerateCommitMessage_SingleRelation(t *testing.T) {
	cs := &ChangeSet{
		Relations: []RelationChange{
			{From: "TKT-001", RelType: "blocks", To: "TKT-002", IsNew: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Link TKT-001 -> TKT-002" {
		t.Errorf("expected 'Link TKT-001 -> TKT-002', got %q", msg)
	}
}

func TestGenerateCommitMessage_MultipleRelations(t *testing.T) {
	cs := &ChangeSet{
		Relations: []RelationChange{
			{From: "TKT-001", RelType: "blocks", To: "TKT-002", IsNew: true},
			{From: "TKT-001", RelType: "blocks", To: "TKT-003", IsNew: true},
		},
	}
	msg := cs.GenerateCommitMessage()
	if msg != "Add 2 relations" {
		t.Errorf("expected 'Add 2 relations', got %q", msg)
	}
}

func TestGenerateCommitMessage_Combined(t *testing.T) {
	cs := &ChangeSet{
		Added: []EntityChange{
			{Type: "ticket", ID: "TKT-003", IsNew: true},
		},
		Modified: []EntityChange{
			{Type: "ticket", ID: "TKT-001", PropsChanged: []string{"status"}},
		},
	}
	msg := cs.GenerateCommitMessage()
	// Should contain both
	if msg != "Add ticket TKT-003; TKT-001: update status" {
		t.Errorf("expected 'Add ticket TKT-003; TKT-001: update status', got %q", msg)
	}
}

func TestGenerateCommitMessage_Empty(t *testing.T) {
	cs := &ChangeSet{}
	msg := cs.GenerateCommitMessage()
	if msg != "Update entities" {
		t.Errorf("expected 'Update entities', got %q", msg)
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
