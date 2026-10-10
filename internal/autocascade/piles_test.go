package autocascade_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestRunner_PushesTriggerOntoPiles(t *testing.T) {
	t.Parallel()
	var got []autocascade.PileRequest
	pusher := autocascade.PilePushFunc(func(_ context.Context, req autocascade.PileRequest) error {
		got = append(got, req)
		if req.Pile == "Broken" {
			return errors.New("piles: unknown owner")
		}
		return nil
	})
	r, err := autocascade.New(autocascade.Deps{Engine: automation.NewEngine(nil), Piles: pusher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	trigger := &entity.Entity{ID: "POL-1", Type: "policy", Face: "draft"}
	outcome, err := r.Process(context.Background(), &stubHost{t: t}, autocascade.Request{
		Trigger: trigger,
		Result: &automation.Result{PilesToPush: []automation.PileToPush{
			{Pile: "Broken", Owner: "PER-X", Create: true, AutomationName: "a"},
			{Pile: "Inbox", Owner: "PER-1", Create: true, AutomationName: "b"},
		}},
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) != 0 {
		t.Errorf("a failed push surfaced as an outcome error: %v", outcome.Errors)
	}
	if len(got) != 2 {
		t.Fatalf("pushes = %+v, want 2 (a failure must not stop the next push)", got)
	}
	want := autocascade.PileRequest{Owner: "PER-1", Pile: "Inbox", Ref: trigger.Ref(), Create: true}
	if got[1] != want {
		t.Errorf("push = %+v, want %+v", got[1], want)
	}
	if got[1].Ref.Face != "draft" {
		t.Errorf("pushed ref lost its face: %s", got[1].Ref)
	}
}

func TestRunner_NoPusherSkipsPiles(t *testing.T) {
	t.Parallel()
	r := newRunner(t, nil)
	outcome, err := r.Process(context.Background(), &stubHost{t: t}, autocascade.Request{
		Trigger: entity.New("TKT-1", "ticket"),
		Result:  &automation.Result{PilesToPush: []automation.PileToPush{{Pile: "Inbox", AutomationName: "a"}}},
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(outcome.Errors) != 0 || len(outcome.Warnings) != 0 {
		t.Errorf("skipping piles changed the outcome: %+v", outcome)
	}
}

// A failed push logs the automation and the entity, never the interpolated
// pile name or owner. Not parallel: it swaps the default logger.
func TestRunner_FailedPushLogOmitsPileAndOwner(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	pusher := autocascade.PilePushFunc(func(context.Context, autocascade.PileRequest) error {
		return errors.New("push failed")
	})
	r, err := autocascade.New(autocascade.Deps{Engine: automation.NewEngine(nil), Piles: pusher})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = r.Process(context.Background(), &stubHost{t: t}, autocascade.Request{
		Trigger: entity.New("TKT-1", "ticket"),
		Result: &automation.Result{PilesToPush: []automation.PileToPush{
			{Pile: "Secret salary 90k", Owner: "alice@example.com", AutomationName: "notify"},
		}},
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"automation=notify", "entity=TKT-1", "push failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q: %s", want, out)
		}
	}
	for _, leak := range []string{"Secret salary", "alice@example.com"} {
		if strings.Contains(out, leak) {
			t.Errorf("log leaks %q: %s", leak, out)
		}
	}
}
