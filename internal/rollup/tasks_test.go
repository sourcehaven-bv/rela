package rollup_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
)

func TestCountTasks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		body        string
		done, total int
	}{
		{"empty", "", 0, 0},
		{"no tasks", "# Notes\n\n- plain item\ntext [x]\n", 0, 0},
		{"open and done", "- [ ] a\n- [x] b\n", 1, 2},
		{"capital X", "- [X] a\n", 1, 1},
		{"star and plus", "* [ ] a\n+ [x] b\n", 1, 2},
		{"numbered", "1. [x] a\n2) [ ] b\n", 1, 2},
		{"nested", "- [ ] parent\n  - [x] child\n    - [x] grandchild\n\t- [ ] tab\n", 2, 4},
		{"crlf", "- [x] a\r\n- [ ] b\r\n", 1, 2},
		{"empty task text", "- [ ]\n- [x]\n", 1, 2},
		{"backtick fence", "- [x] a\n```\n- [ ] in code\n- [x] in code\n```\n- [ ] b\n", 1, 2},
		{"tilde fence", "~~~md\n- [ ] in code\n~~~\n- [x] a\n", 1, 1},
		{"longer closing fence", "````\n- [ ] in\n```\n- [ ] still in\n`````\n- [x] out\n", 1, 1},
		{"fence with info is not a close", "```\n- [ ] in\n```go\n- [ ] in\n```\n", 0, 0},
		{"tilde does not close backtick", "```\n~~~\n- [ ] in\n```\n- [ ] out\n", 0, 1},
		{"unclosed fence", "- [x] a\n```\n- [ ] in\n", 1, 1},
		{"indented fence", "- item\n  ```\n  - [ ] in\n  ```\n", 0, 0},
		{"no space after marker", "-[ ] a\n1.[x] b\n", 0, 0},
		{"other box char", "- [-] a\n- [y] b\n", 0, 0},
		{"box glued to text", "- [x]done\n", 0, 0},
		{"link-like", "- [link](x)\n", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			done, total := rollup.CountTasks(tc.body)
			if done != tc.done || total != tc.total {
				t.Errorf("CountTasks = %d/%d, want %d/%d", done, total, tc.done, tc.total)
			}
		})
	}
}

func TestCompile_BodyTasks(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  todo:
    label: Todo
    id_prefix: "TODO-"
    properties:
      title: {type: string}
      tasks_done: {type: integer, rollup: {body_tasks: done}}
      tasks_open: {type: integer, rollup: {body_tasks: open}}
      tasks_total: {type: integer, rollup: {body_tasks: total}}
`))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	body := "- [x] a\n- [ ] b\n- [ ] c\n"
	want := map[string]int64{"tasks_done": 1, "tasks_open": 2, "tasks_total": 3}
	cols := set.For("todo")
	if len(cols) != len(want) {
		t.Fatalf("columns = %v", cols)
	}
	for _, c := range cols {
		if c.Func != metamodel.RollupBodyTasks {
			t.Errorf("%s: func %q", c.Property, c.Func)
		}
		if got := c.TaskCount(body); got != want[c.Property] {
			t.Errorf("%s = %d, want %d", c.Property, got, want[c.Property])
		}
	}
}

func TestParse_BodyTasksShape(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, prop string }{
		{"unknown count", `{type: integer, rollup: {body_tasks: some}}`},
		{"not integer", `{type: string, rollup: {body_tasks: done}}`},
		{"with via", `{type: integer, rollup: {body_tasks: done, via: x}}`},
		{"with records", `{type: integer, rollup: {body_tasks: done, records: x}}`},
		{"with count", `{type: integer, rollup: {body_tasks: done, count: true}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := metamodel.Parse([]byte(`version: "1.0"
entities:
  todo:
    label: Todo
    id_prefix: "TODO-"
    properties:
      title: {type: string}
      n: ` + tc.prop + "\n"))
			if err == nil {
				t.Error("Parse accepted the declaration")
			}
		})
	}
}
