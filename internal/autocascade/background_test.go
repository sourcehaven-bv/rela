package autocascade_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

type recordingBackground struct {
	got []autocascade.BackgroundScript
	err error
}

func (r *recordingBackground) EnqueueScript(_ context.Context, s autocascade.BackgroundScript) error {
	r.got = append(r.got, s)
	return r.err
}

func backgroundRequest() autocascade.Request {
	trigger := entity.New("NOTE-1", "note")
	trigger.Face = "draft"
	return autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{LuaToExecute: []automation.LuaToExecute{
			{FilePath: "push.lua", AutomationName: "push", Background: true},
		}},
	}
}

// TestRunnerEnqueuesBackgroundAction: a background action is handed to the
// scheduler with the trigger's face, never to the script runner.
func TestRunnerEnqueuesBackgroundAction(t *testing.T) {
	t.Parallel()
	bg := &recordingBackground{}
	r, err := autocascade.New(autocascade.Deps{Engine: automation.NewEngine(nil), Background: bg})
	if err != nil {
		t.Fatal(err)
	}
	req := backgroundRequest()
	req.Scripts = &failingScriptRunner{err: errors.New("must not run inline")}
	outcome, err := r.Process(context.Background(), &stubHost{t: t}, req)
	if err != nil || len(outcome.Errors) != 0 {
		t.Fatalf("Process: %v %v", err, outcome.Errors)
	}
	want := autocascade.BackgroundScript{Automation: "push", LuaFile: "push.lua", Ref: entity.Ref{ID: "NOTE-1", Face: "draft"}}
	if len(bg.got) != 1 || bg.got[0] != want {
		t.Errorf("enqueued %+v, want %+v", bg.got, want)
	}
}

// TestRunnerBackgroundErrors: a missing scheduler or a refused enqueue is an
// outcome error; the save stands.
func TestRunnerBackgroundErrors(t *testing.T) {
	t.Parallel()
	for name, bg := range map[string]autocascade.BackgroundScripts{
		"none":    nil,
		"refused": &recordingBackground{err: errors.New("queue closed")},
	} {
		r, err := autocascade.New(autocascade.Deps{Engine: automation.NewEngine(nil), Background: bg})
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := r.Process(context.Background(), &stubHost{t: t}, backgroundRequest())
		if err != nil || len(outcome.Errors) != 1 {
			t.Errorf("%s: err=%v errors=%v", name, err, outcome.Errors)
		}
	}
}
