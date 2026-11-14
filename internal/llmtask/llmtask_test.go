package llmtask

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func testMeta() *metamodel.Metamodel {
	lo, hi, two := 1, 5, 2
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{"size": {Values: []string{"s", "m", "l"}}},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title":    {Type: metamodel.PropertyTypeString},
				"priority": {Type: metamodel.PropertyTypeEnum, Values: []string{"low", "high"}, Description: "How urgent"},
				"effort":   {Type: "size"},
				"points":   {Type: metamodel.PropertyTypeInteger, Min: &lo, Max: &hi},
				"done":     {Type: metamodel.PropertyTypeBoolean},
				"due":      {Type: metamodel.PropertyTypeDate},
				"at":       {Type: metamodel.PropertyTypeDatetime},
				"cost":     {Type: metamodel.PropertyTypeDecimal, Scale: &two},
				"tags":     {Type: metamodel.PropertyTypeString, List: true},
				"secret":   {Type: metamodel.PropertyTypeString},
			}},
			"concept": {Properties: map[string]metamodel.PropertyDef{"title": {Type: metamodel.PropertyTypeString}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"affects": {From: []string{"ticket"}, To: []string{"concept"}, Description: "Concepts the ticket touches"},
		},
	}
}

func testTask(t *testing.T) *Task {
	t.Helper()
	action := dataentryconfig.Action{
		Permission:  "p",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
		LLM: &dataentryconfig.LLMAction{
			Prompt:    "Triage it.",
			MaxTokens: 500,
			Inputs: dataentryconfig.LLMInputs{
				Properties: []string{"title", "secret"},
				Content:    &dataentryconfig.LLMContentInput{MaxChars: 5},
				Related:    []dataentryconfig.LLMRelatedInput{{Relation: "affects"}},
			},
			Outputs: dataentryconfig.LLMOutputs{
				Set:    []string{"priority", "effort", "points", "done", "due", "at", "cost", "tags"},
				Relate: []dataentryconfig.LLMRelateOutput{{Relation: "affects", Max: 2}},
			},
		},
	}
	task, err := New("triage", action, testMeta())
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func testInput() Input {
	return Input{
		Subject: &entity.Entity{
			ID: "TKT-1", Type: "ticket", Content: "héllo world",
			// secret is absent: the caller's read redacted it.
			Properties: map[string]any{"title": "Login fails", "priority": "low", "tags": []string{"a"}},
		},
		Related: [][]Item{{{ID: "C-1", Type: "concept", Title: "Auth"}}},
		Candidates: []Candidates{{Items: []Item{
			{ID: "C-1", Type: "concept", Title: "Auth", Related: true},
			{ID: "C-2", Type: "concept", Title: "Login"},
			{ID: "C-3", Type: "concept", Title: "Billing"},
		}}},
		Writable:  []string{"priority", "effort", "points", "done", "due", "at", "cost", "tags"},
		Relatable: []string{"affects"},
	}
}

func TestNew_Rejects(t *testing.T) {
	if _, err := New("x", dataentryconfig.Action{}, nil); err == nil {
		t.Error("nil meta accepted")
	}
	if _, err := New("x", dataentryconfig.Action{}, testMeta()); err == nil {
		t.Error("non-llm action accepted")
	}
	bad := dataentryconfig.Action{LLM: &dataentryconfig.LLMAction{},
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"nope"}}}
	if _, err := New("x", bad, testMeta()); err == nil {
		t.Error("unknown subject accepted")
	}
}

func TestRequest_Shape(t *testing.T) {
	task := testTask(t)
	req, err := task.Request(testInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
		t.Fatalf("messages = %+v", req.Messages)
	}
	if !strings.Contains(req.Messages[0].Content, "Triage it.") {
		t.Error("prompt missing from system message")
	}
	if strings.Contains(req.Messages[0].Content, "Login fails") {
		t.Error("entity data leaked into the system message")
	}
	if req.MaxTokens == nil || *req.MaxTokens != 500 {
		t.Errorf("max tokens = %v", req.MaxTokens)
	}
	if req.ResponseFormat == nil || !req.ResponseFormat.Strict {
		t.Fatal("no strict response format")
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &doc); err != nil {
		t.Fatalf("user message is not JSON: %v", err)
	}
	rec := doc["record"].(map[string]any)
	if rec["content"] != "héllo" || rec["content_truncated"] != true {
		t.Errorf("content = %v, truncated = %v", rec["content"], rec["content_truncated"])
	}
	props := rec["properties"].(map[string]any)
	if _, ok := props["secret"]; ok {
		t.Error("redacted property sent")
	}
	if props["priority"] != "low" {
		t.Error("current value of an output property not sent")
	}
	if doc["related"] == nil || doc["candidates"] == nil {
		t.Errorf("related/candidates missing: %v", doc)
	}
}

func TestRequest_CapsLongValues(t *testing.T) {
	long := strings.Repeat("é", MaxValueChars+10)
	in := testInput()
	in.Subject.Properties["title"] = long
	relSrc := &entity.Entity{ID: "C-1", Type: "concept", Properties: map[string]any{"title": long, "n": 3}}
	in.Related[0][0] = NewItem(testMeta(), relSrc, []string{"title", "n"})
	cand := &entity.Entity{ID: "C-2", Type: "concept", Properties: map[string]any{"title": []any{"short", long}}}
	in.Candidates[0].Items[1] = NewItem(testMeta(), cand, []string{"title"})

	req, err := testTask(t).Request(in)
	if err != nil {
		t.Fatal(err)
	}
	type wireItem struct {
		Properties map[string]any `json:"properties"`
		Truncated  []string       `json:"truncated_fields"`
	}
	var doc struct {
		Record  wireItem `json:"record"`
		Related []struct {
			Entities []wireItem `json:"entities"`
		} `json:"related"`
		Candidates map[string]struct {
			Entities []wireItem `json:"entities"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &doc); err != nil {
		t.Fatal(err)
	}
	capped := strings.Repeat("é", MaxValueChars)
	if doc.Record.Properties["title"] != capped || !slices.Equal(doc.Record.Truncated, []string{"title"}) {
		t.Errorf("subject title not capped: truncated = %v", doc.Record.Truncated)
	}
	rel := doc.Related[0].Entities[0]
	if rel.Properties["title"] != capped || rel.Properties["n"] != float64(3) ||
		!slices.Equal(rel.Truncated, []string{"title"}) {

		t.Errorf("related item = %+v", rel.Truncated)
	}
	candDoc := doc.Candidates["affects"].Entities[1]
	if got := candDoc.Properties["title"].([]any); got[0] != "short" || got[1] != capped {
		t.Errorf("list value not capped per element")
	}
	// The caller's input is not modified.
	if relSrc.Properties["title"] != long {
		t.Error("input item was modified")
	}
}

func TestRequest_NothingToPropose(t *testing.T) {
	in := testInput()
	in.Writable, in.Relatable = nil, nil
	if _, err := testTask(t).Request(in); !errors.Is(err, ErrNothingToPropose) {
		t.Fatalf("err = %v", err)
	}
}

func TestRequest_InputMismatch(t *testing.T) {
	in := testInput()
	in.Related = nil
	if _, err := testTask(t).Request(in); err == nil {
		t.Fatal("mismatched input accepted")
	}
	if _, err := testTask(t).Request(Input{}); err == nil {
		t.Fatal("input without subject accepted")
	}
}

func TestSchema(t *testing.T) {
	in := testInput()
	in.Writable = []string{"priority", "effort", "points", "tags"}
	s := testTask(t).Schema(in)
	set := s["properties"].(map[string]any)["set"].(map[string]any)
	if got := set["required"].([]string); strings.Join(got, ",") != "priority,effort,points,tags" {
		t.Errorf("required = %v", got)
	}
	props := set["properties"].(map[string]any)
	if _, ok := props["done"]; ok {
		t.Error("unwritable property in schema")
	}
	prio := props["priority"].(map[string]any)
	if prio["description"] != "How urgent" {
		t.Errorf("description = %v", prio["description"])
	}
	val := prio["anyOf"].([]any)[0].(map[string]any)
	if strings.Join(val["enum"].([]string), ",") != "low,high" {
		t.Errorf("enum = %v", val["enum"])
	}
	effort := props["effort"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)
	if strings.Join(effort["enum"].([]string), ",") != "s,m,l" {
		t.Errorf("custom enum = %v", effort["enum"])
	}
	points := props["points"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)
	if points["minimum"] != 1 || points["maximum"] != 5 {
		t.Errorf("points = %v", points)
	}
	tags := props["tags"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)
	if tags["type"] != "array" {
		t.Errorf("tags = %v", tags)
	}

	rel := s["properties"].(map[string]any)["relate"].(map[string]any)["properties"].(map[string]any)["affects"].(map[string]any)
	if rel["maxItems"] != 2 || strings.Join(rel["items"].(map[string]any)["enum"].([]string), ",") != "C-1,C-2,C-3" {
		t.Errorf("relate = %v", rel)
	}

	in.Candidates[0].Items = nil
	rel = testTask(t).Schema(in)["properties"].(map[string]any)["relate"].(map[string]any)["properties"].(map[string]any)["affects"].(map[string]any)
	if rel["maxItems"] != 0 {
		t.Errorf("empty candidates maxItems = %v", rel["maxItems"])
	}
}

func reply(content string) *ai.ChatResponse {
	return &ai.ChatResponse{Content: content, Model: "m", FinishReason: "stop"}
}

func TestParse_Valid(t *testing.T) {
	content := "```json\n" + `{"set":{"priority":"high","effort":null,"points":3,"done":true,
		"due":"2026-12-01","at":"2026-12-01T10:00:00Z","cost":12.50,"tags":["a"]},
		"relate":{"affects":["C-1","C-2","C-2"]}}` + "\n```"
	p, err := testTask(t).Parse(reply(content), testInput())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range p.Set {
		names = append(names, c.Property)
	}
	// effort is null, tags is unchanged: both left out. Config order kept.
	if strings.Join(names, ",") != "priority,points,done,due,at,cost" {
		t.Errorf("set = %v", names)
	}
	if string(p.Set[0].Current) != `"low"` || string(p.Set[0].Proposed) != `"high"` {
		t.Errorf("priority change = %+v", p.Set[0])
	}
	if string(p.Set[5].Proposed) != `"12.50"` {
		t.Errorf("cost = %s", p.Set[5].Proposed)
	}
	// C-1 is already related; C-2 is deduplicated.
	if len(p.Relate) != 1 || p.Relate[0].To != "C-2" || p.Relate[0].ToType != "concept" {
		t.Errorf("relate = %+v", p.Relate)
	}
	if p.Model != "m" {
		t.Errorf("model = %q", p.Model)
	}
}

func TestParse_Rejects(t *testing.T) {
	tests := []struct {
		name, content, want string
		finish              string
	}{
		{name: "not json", content: "sure!", want: "not the JSON"},
		{name: "extra top-level key", content: `{"set":{},"relate":{},"x":1}`, want: "not the JSON"},
		{name: "trailing value", content: `{"set":{},"relate":{}} {"set":{}}`, want: "not the JSON"},
		{name: "cut off", content: `{"set"`, finish: "length", want: "cut off"},
		{name: "unknown property", content: `{"set":{"title":"x"},"relate":{}}`, want: `property "title" is not one`},
		{name: "invalid enum", content: `{"set":{"priority":"urgent"},"relate":{}}`, want: `"urgent" is not an allowed value`},
		{name: "integer out of range", content: `{"set":{"points":9},"relate":{}}`, want: "out of range"},
		{name: "integer not whole", content: `{"set":{"points":1.5},"relate":{}}`, want: "whole number"},
		{name: "bad bool", content: `{"set":{"done":"yes"},"relate":{}}`, want: "true or false"},
		{name: "bad date", content: `{"set":{"due":"tomorrow"},"relate":{}}`, want: "not a date"},
		{name: "bad datetime", content: `{"set":{"at":"2026-12-01"},"relate":{}}`, want: "not a date-time"},
		{name: "bad decimal", content: `{"set":{"cost":"lots"},"relate":{}}`, want: "decimal"},
		{name: "decimal beyond scale", content: `{"set":{"cost":"1.234"},"relate":{}}`, want: `property "cost"`},
		{name: "list not list", content: `{"set":{"tags":"a"},"relate":{}}`, want: "expected a list"},
		{name: "text not text", content: `{"set":{"priority":1},"relate":{}}`, want: "expected text"},
		{name: "unknown relation", content: `{"set":{},"relate":{"owns":[]}}`, want: `relation "owns" is not one`},
		{name: "id outside candidates", content: `{"set":{},"relate":{"affects":["C-9"]}}`, want: `"C-9" is not one of the candidates`},
		{name: "too many", content: `{"set":{},"relate":{"affects":["C-1","C-2","C-3"]}}`, want: "at most 2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := reply(tc.content)
			if tc.finish != "" {
				r.FinishReason = tc.finish
			}
			_, err := testTask(t).Parse(r, testInput())
			var re *ReplyError
			if !errors.As(err, &re) {
				t.Fatalf("err = %v, want a ReplyError", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := testTask(t).Parse(nil, testInput()); err == nil {
		t.Error("nil reply accepted")
	}
}

func TestParse_UnwritableAndUnrelatableRefused(t *testing.T) {
	in := testInput()
	in.Writable = []string{"effort"}
	in.Relatable = nil
	task := testTask(t)
	if _, err := task.Parse(reply(`{"set":{"priority":"high"},"relate":{}}`), in); err == nil {
		t.Error("unwritable property accepted")
	}
	if _, err := task.Parse(reply(`{"set":{},"relate":{"affects":["C-2"]}}`), in); err == nil {
		t.Error("unrelatable relation accepted")
	}
}

func TestTruncate(t *testing.T) {
	if s, cut := truncate("abc", 3); s != "abc" || cut {
		t.Errorf("truncate = %q %v", s, cut)
	}
	if s, cut := truncate("ééé", 2); s != "éé" || !cut {
		t.Errorf("truncate = %q %v", s, cut)
	}
}

func TestSameValue(t *testing.T) {
	if !sameValue([]string{"a"}, []any{"a"}) {
		t.Error("[]string vs []any")
	}
	if !sameValue(int(3), int64(3)) {
		t.Error("int vs int64")
	}
	if sameValue(nil, "x") {
		t.Error("nil vs x")
	}
}
