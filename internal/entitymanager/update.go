package entitymanager

import (
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// UpdateWithRelations atomically updates an entity's properties + content
// + outgoing relations in a single transaction.
//
// On success returns the post-commit entity, the set of counterparty
// IDs touched by symmetric/inverse propagation, and any automation
// side-effect bookkeeping. The caller is responsible for fanning out
// observability events (SSE, webhooks, etc.).
//
// Errors:
//   - ErrEntityNotFound: entity doesn't exist or its type doesn't match.
//   - ErrETagMismatch: optimistic-concurrency check failed.
//   - *RequestShapeError: caller-side translation issue (HTTP: 400).
//   - *ValidationError: metamodel rejected the request (HTTP: 422).
//   - other errors: storage / commit failure.
func (m *Manager) UpdateWithRelations(req UpdateWithRelationsRequest) (*UpdateWithRelationsResult, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}

	// Fast-path 404 outside the transaction. The race that the in-tx
	// re-lookup guards against (entity deleted between this check and
	// the staged write) does not apply when the entity doesn't exist
	// here — there's nothing to race with.
	if _, ok := m.ws.Snapshot().GetEntity(req.EntityID); !ok {
		return nil, ErrEntityNotFound
	}

	st := &updateState{}
	st.diff.counterparties = make(map[string]struct{})

	if err := m.ws.WithTx(func(tx *workspace.Tx) error {
		return m.runUpdateTx(tx, req, st)
	}); err != nil {
		return nil, passthroughTxError(err)
	}

	return m.buildResult(req, st), nil
}

// updateState carries everything the WithTx callback produces that the
// post-commit phase needs to read. Using a struct keeps the helpers'
// signatures honest about what they read and write.
type updateState struct {
	staged           *model.Entity
	preStageEntity   *model.Entity
	diff             relationDiff
	hadWrites        bool
	automationResult *automation.Result
}

func (req *UpdateWithRelationsRequest) validate() error {
	if req.EntityID == "" {
		return requestShapeErrorf("entity_id is required")
	}
	if req.IfMatch != "" && req.ETagFn == nil {
		return requestShapeErrorf("if_match requires ETagFn")
	}
	return nil
}

// runUpdateTx is the WithTx-callback body.
func (m *Manager) runUpdateTx(tx *workspace.Tx, req UpdateWithRelationsRequest, st *updateState) error {
	live, err := lookupLive(tx, req)
	if err != nil {
		return err
	}
	st.preStageEntity = live.Clone()
	st.staged = live.Clone()

	meta := tx.Meta()
	if err := applyEntityChanges(meta, st.staged, req); err != nil {
		return err
	}

	if err := computeDiff(tx, meta, req.EntityID, live.Type, req.Relations, &st.diff); err != nil {
		return err
	}
	propagateRelations(tx, meta, &st.diff)
	if err := validateStagedEdges(tx, meta, st.diff.adds); err != nil {
		return err
	}
	if err := validateEntity(meta, st.staged); err != nil {
		return err
	}

	// Synchronous automation hooks (property sets) inside the tx so
	// any property changes they apply land atomically. Side effects
	// defer to post-commit.
	st.automationResult = tx.RunEntityUpdateAutomation(st.staged, st.preStageEntity)

	return commitStagedWrites(tx, st)
}

// lookupLive re-fetches the entity inside the transaction (so reloadMu
// serializes us against file-watcher reloads), checks the expected
// type, and verifies the optional If-Match.
func lookupLive(tx *workspace.Tx, req UpdateWithRelationsRequest) (*model.Entity, error) {
	live, ok := tx.GetEntity(req.EntityID)
	if !ok {
		return nil, ErrEntityNotFound
	}
	if req.ExpectedType != "" && live.Type != req.ExpectedType {
		return nil, ErrEntityNotFound
	}
	if req.IfMatch != "" && req.IfMatch != req.ETagFn(live) {
		return nil, ErrETagMismatch
	}
	return live, nil
}

// applyEntityChanges merges the request's property and content changes
// into staged, after rejecting unknown PropertiesUnset keys against the
// entity type's closed schema.
func applyEntityChanges(meta *metamodel.Metamodel, staged *model.Entity, req UpdateWithRelationsRequest) error {
	if entDef, ok := meta.GetEntityDef(staged.Type); ok && len(req.PropertiesUnset) > 0 {
		for _, k := range req.PropertiesUnset {
			if _, known := entDef.Properties[k]; !known {
				return validationErrorf(
					"/properties_unset: unknown property %q for entity type %q", k, staged.Type)
			}
		}
	}
	for k, v := range req.Properties {
		staged.Properties[k] = v
	}
	for _, k := range req.PropertiesUnset {
		delete(staged.Properties, k)
	}
	if req.Content != nil {
		staged.Content = *req.Content
	}
	return nil
}

// validateEntity runs the metamodel's full entity validation against
// the staged entity, concatenating multiple errors into a single
// detail string.
func validateEntity(meta *metamodel.Metamodel, staged *model.Entity) error {
	errs := meta.ValidateEntity(staged)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Message
	}
	return validationErrorf("validation: %s", strings.Join(msgs, "; "))
}

// commitStagedWrites detects whether anything changed and stages the
// corresponding writes. Sets st.hadWrites accordingly.
func commitStagedWrites(tx *workspace.Tx, st *updateState) error {
	entityChanged := !entitiesEqual(st.preStageEntity, st.staged)
	if !entityChanged && len(st.diff.adds) == 0 && len(st.diff.removes) == 0 {
		return nil
	}
	st.hadWrites = true

	if entityChanged {
		if err := tx.WriteEntity(st.staged); err != nil {
			return err
		}
	}
	for _, rel := range st.diff.adds {
		if err := tx.WriteRelation(rel); err != nil {
			return err
		}
	}
	for _, rem := range st.diff.removes {
		if err := tx.DeleteRelation(rem.from, rem.relType, rem.to); err != nil {
			return err
		}
	}
	return nil
}

// passthroughTxError unwraps the typed manager errors so callers can
// switch on errors.Is / errors.As at the top level.
func passthroughTxError(err error) error {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}
	var rse *RequestShapeError
	if errors.As(err, &rse) {
		return rse
	}
	return err
}

// buildResult assembles the post-commit result, runs the deferred
// automation side effects (relation/entity creation, Lua) outside the
// transaction, and (on a no-op) returns the live entity unchanged.
func (m *Manager) buildResult(req UpdateWithRelationsRequest, st *updateState) *UpdateWithRelationsResult {
	result := &UpdateWithRelationsResult{NoOp: !st.hadWrites}

	if st.automationResult != nil && st.hadWrites {
		effects := m.ws.ApplyAutomationSideEffectsAfterCommit(st.staged, st.preStageEntity, st.automationResult)
		if effects != nil {
			result.RelationsCreated = effects.RelationsCreated
			result.EntitiesCreated = effects.EntitiesCreated
			result.AutomationWarnings = append(result.AutomationWarnings, effects.Warnings...)
			result.AutomationErrors = append(result.AutomationErrors, effects.Errors...)
		}
	}

	if !st.hadWrites {
		live, _ := m.ws.Snapshot().GetEntity(req.EntityID)
		if live == nil {
			live = st.preStageEntity
		}
		result.Entity = live
		return result
	}

	result.Entity = st.staged
	for cpID := range st.diff.counterparties {
		if cpID == req.EntityID {
			continue
		}
		result.Counterparties = append(result.Counterparties, cpID)
	}
	return result
}
