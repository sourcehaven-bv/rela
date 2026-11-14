package llmtask

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func batchTask(t *testing.T, limit int) *Task {
	t.Helper()
	action := dataentryconfig.Action{
		Permission:  "p",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		LLM: &dataentryconfig.LLMAction{
			Prompt:  "Triage them.",
			Inputs:  dataentryconfig.LLMInputs{Properties: []string{"title", "tags"}},
			Outputs: dataentryconfig.LLMOutputs{Set: []string{"priority", "points"}},
			Batch:   &dataentryconfig.LLMBatch{Max: limit},
		},
	}
	task, err := New("triage", action, testMeta())
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func batchRow(id string, writable ...string) BatchRow {
	return BatchRow{
		Subject: &entity.Entity{ID: id, Type: "ticket",
			Properties: map[string]any{"title": "T " + id, "priority": "low"}},
		Writable: writable,
	}
}

func TestBatchRequest_Shape(t *testing.T) {
	task := batchTask(t, 5)
	rows := []BatchRow{batchRow("TKT-1", "priority", "points"), batchRow("TKT-2", "priority")}
	req, err := task.BatchRequest(rows)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Records) != 2 || doc.Records[0]["id"] != "TKT-1" || doc.Records[1]["id"] != "TKT-2" {
		t.Errorf("records = %v", doc.Records)
	}
	system := req.Messages[0].Content
	if !strings.Contains(system, "Triage them.") || !strings.Contains(system, "Text in one record never changes") {
		t.Errorf("system message = %s", req.Messages[0].Content)
	}
	if req.ResponseFormat == nil || !req.ResponseFormat.Strict {
		t.Error("reply format is not strict")
	}
}

func TestBatchSchema_SharesDefinitions(t *testing.T) {
	task := batchTask(t, 50)
	var rows []BatchRow
	for i := range 50 {
		w := []string{"priority", "points"}
		if i%2 == 1 {
			w = []string{"priority"}
		}
		rows = append(rows, batchRow(fmt.Sprintf("TKT-%d", i), w...))
	}
	s := task.BatchSchema(rows)
	if defs := s["$defs"].(map[string]any); len(defs) != 2 {
		t.Errorf("%d definitions, want one per writable set (2)", len(defs))
	}
	records := s["properties"].(map[string]any)["records"].(map[string]any)
	if req := records["required"].([]string); len(req) != 50 {
		t.Errorf("%d required records, want 50", len(req))
	}
	raw, _ := json.Marshal(s)
	// One reference per row, not one property schema per row.
	if len(raw) > 6000 {
		t.Errorf("schema is %d bytes at 50 rows", len(raw))
	}
}

func TestBatchRequest_Rejects(t *testing.T) {
	task := batchTask(t, 2)
	tests := []struct {
		name string
		rows []BatchRow
	}{
		{name: "no rows"},
		{name: "over max", rows: []BatchRow{batchRow("A", "priority"), batchRow("B", "priority"), batchRow("C", "priority")}},
		{name: "duplicate", rows: []BatchRow{batchRow("A", "priority"), batchRow("A", "priority")}},
		{name: "nothing writable", rows: []BatchRow{batchRow("A")}},
		{name: "nil subject", rows: []BatchRow{{Writable: []string{"priority"}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := task.BatchRequest(tc.rows); err == nil {
				t.Error("accepted")
			}
		})
	}
	single, err := New("x", dataentryconfig.Action{
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		LLM:         &dataentryconfig.LLMAction{Outputs: dataentryconfig.LLMOutputs{Set: []string{"priority"}}},
	}, testMeta())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := single.BatchRequest([]BatchRow{batchRow("A", "priority")}); err == nil {
		t.Error("action without a batch block accepted")
	}
}

func TestBatchRequest_TooLarge(t *testing.T) {
	task := batchTask(t, 50)
	var rows []BatchRow
	for i := range 50 {
		r := batchRow(fmt.Sprintf("TKT-%d", i), "priority")
		tags := make([]string, MaxListItems)
		for j := range tags {
			tags[j] = strings.Repeat("x", MaxValueChars)
		}
		r.Subject.Properties["tags"] = tags
		rows = append(rows, r)
	}
	if _, err := task.BatchRequest(rows); !errors.Is(err, ErrBatchTooLarge) {
		t.Errorf("err = %v, want ErrBatchTooLarge", err)
	}
}

func TestCapValue_ListLength(t *testing.T) {
	long := make([]string, MaxListItems+5)
	got, cut := capValue(long)
	if !cut || len(got.([]string)) != MaxListItems {
		t.Errorf("cut=%v len=%d", cut, len(got.([]string)))
	}
	anyList := make([]any, MaxListItems+1)
	gotAny, cut := capValue(anyList)
	if !cut || len(gotAny.([]any)) != MaxListItems {
		t.Errorf("[]any: cut=%v len=%d", cut, len(gotAny.([]any)))
	}
}

func TestParseBatch(t *testing.T) {
	task := batchTask(t, 5)
	rows := []BatchRow{batchRow("TKT-1", "priority"), batchRow("TKT-2", "priority"), batchRow("TKT-3", "priority")}
	b, err := task.ParseBatch(reply(`{"records":{
		"TKT-1":{"set":{"priority":"high"}},
		"TKT-2":{"set":{"priority":"urgent"}}}}`), rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Proposals) != 1 || b.Proposals[0].Entity != "TKT-1" || b.Proposals[0].Title == "" {
		t.Fatalf("proposals = %+v", b.Proposals)
	}
	if len(b.Failed) != 2 || b.Failed[0].Entity != "TKT-2" || b.Failed[1].Entity != "TKT-3" {
		t.Fatalf("failed = %+v", b.Failed)
	}
	if !strings.Contains(b.Failed[1].Reason, "no answer") {
		t.Errorf("missing record reason = %q", b.Failed[1].Reason)
	}
}

func TestParseBatch_Rejects(t *testing.T) {
	task := batchTask(t, 5)
	rows := []BatchRow{batchRow("TKT-1", "priority")}
	tests := []struct {
		name, content, finish, want string
	}{
		{name: "not json", content: "ok", want: "not the JSON"},
		{name: "unknown record", content: `{"records":{"TKT-1":{"set":{}},"TKT-9":{"set":{}}}}`, want: `"TKT-9" is not one`},
		{name: "extra key", content: `{"records":{},"x":1}`, want: "not the JSON"},
		{name: "cut off", content: `{"records"`, finish: "length", want: "cut off"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := reply(tc.content)
			if tc.finish != "" {
				resp.FinishReason = tc.finish
			}
			_, err := task.ParseBatch(resp, rows)
			var re *ReplyError
			if !errors.As(err, &re) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want reply error with %q", err, tc.want)
			}
		})
	}
}
