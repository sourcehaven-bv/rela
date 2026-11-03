package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// blockingMutator refuses every delete the way the channel guard does.
type blockingMutator struct{ entityMutator }

func (blockingMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return nil, &entitymanager.AnchorBlockedError{ChannelType: "incident-room"}
}

func (blockingMutator) DeleteEntityFace(context.Context, string, entity.Face, bool) (*entity.DeleteResult, error) {
	return nil, &entitymanager.AnchorBlockedError{ChannelType: "incident-room"}
}

// conflictingMutator fails every delete the way a delete that raced a face
// or owned-entity change does.
type conflictingMutator struct{ entityMutator }

var errRaced = fmt.Errorf("delete: the entities it owns changed during the delete: %w",
	entitymanager.ErrConcurrentChange)

func (conflictingMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return nil, errRaced
}

func (conflictingMutator) DeleteEntityFace(context.Context, string, entity.Face, bool) (*entity.DeleteResult, error) {
	return nil, errRaced
}

// A delete that raced a concurrent change is a retryable 409.
func TestV1DeleteEntity_ConflictIs409(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
	app.write.manager = conflictingMutator{entityMutator: app.write.manager}
	app.write.softDeletes = nil

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	app.write.handleV1DeleteEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d, want 409: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "conflict") || !strings.Contains(body, "retry") {
		t.Errorf("body %s should carry the conflict code and a retry hint", body)
	}
}

type blockingSoftDeleter struct{ softDeleter }

func (blockingSoftDeleter) SoftDeleteEntity(context.Context, string) (*entity.DeleteResult, error) {
	return nil, &entitymanager.AnchorBlockedError{ChannelType: "incident-room"}
}

// A delete a channel type blocks is a 409 naming the type, on the hard and
// the soft delete path alike, and the entity stays.
func TestV1DeleteEntity_AnchorBlockedIs409(t *testing.T) {
	for _, soft := range []bool{false, true} {
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
		app.write.manager = blockingMutator{entityMutator: app.write.manager}
		app.write.softDeletes = nil
		if soft {
			app.write.softDeletes = blockingSoftDeleter{}
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tickets/TKT-001", http.NoBody)
		rec := httptest.NewRecorder()
		app.write.handleV1DeleteEntity(rec, req, "ticket", "tickets", "TKT-001")

		if rec.Code != http.StatusConflict {
			t.Fatalf("soft=%v: got %d, want 409: %s", soft, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "channel_blocks_delete") || !strings.Contains(body, "incident-room") {
			t.Errorf("soft=%v: body %s should carry the code and name the channel type", soft, body)
		}
	}
}
