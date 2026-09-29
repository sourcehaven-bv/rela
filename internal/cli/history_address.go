package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// historyAddress resolves the address a history, restore or purge command was
// given to ONE face lineage (BUG-4SYAA6).
//
// A lineage is keyed by (id, face): a draft and its published face have
// separate histories. `ID@face` names the lineage directly. A bare id names
// the zero face, which is the only face of a type without `faces:`. A faced
// type has no zero-face row (DEC-NPZICR), so a bare id of a faced entity would
// silently read an empty lineage, or purge nothing, while the operator
// believes they addressed the entity. It is refused instead, and the error
// names the faces the id has, live or in history.
func historyAddress(
	ctx context.Context, st store.EntityLister, meta *metamodel.Metamodel,
	versions store.StateHistoryReader, raw string,
) (entity.Ref, error) {
	ref, err := entity.ParseRef(raw)
	if err != nil {
		return entity.Ref{}, fmt.Errorf("invalid entity address %q: %w", raw, err)
	}
	if !ref.Face.IsDefault() {
		return ref, nil
	}

	faces, err := liveFacesOf(ctx, st, ref.ID)
	if err != nil {
		return entity.Ref{}, err
	}
	if slices.Contains(faces, "") {
		return ref, nil
	}
	if len(faces) == 0 {
		zero, err := versions.ListStateVersions(ctx, ref.ID, "")
		if err != nil {
			return entity.Ref{}, fmt.Errorf("read history for %q: %w", ref.ID, err)
		}
		if len(zero) > 0 {
			return ref, nil
		}
	}

	// No zero-face row and no zero-face history: the faces the id has are
	// its live faces plus every declared face with a history.
	for _, face := range declaredFaces(meta) {
		if slices.Contains(faces, face) {
			continue
		}
		metas, err := versions.ListStateVersions(ctx, ref.ID, face)
		if err != nil {
			return entity.Ref{}, fmt.Errorf("read history for %q: %w", entity.FormatStateRef(ref.ID, face), err)
		}
		if len(metas) > 0 {
			faces = append(faces, face)
		}
	}
	if len(faces) == 0 {
		return ref, nil
	}
	slices.Sort(faces)
	named := make([]string, len(faces))
	for i, face := range faces {
		named[i] = entity.FormatStateRef(ref.ID, face)
	}
	return entity.Ref{}, fmt.Errorf("%s has faces, and each face has its own history; name one: %s",
		ref.ID, strings.Join(named, ", "))
}

// liveFacesOf returns the faces id has a live row at, from one raw header
// read. The operator shell reads without ACL, like every other CLI command.
func liveFacesOf(ctx context.Context, st store.EntityLister, id string) ([]entity.Face, error) {
	_, faces, err := storedFamily(ctx, st, id)
	return faces, err
}

// declaredFaces returns every face any type declares, each once, sorted. A
// deleted id has no row left to say what type it was, so its history is
// probed at each of them.
func declaredFaces(meta *metamodel.Metamodel) []entity.Face {
	var faces []entity.Face
	if meta == nil {
		return nil
	}
	for _, def := range meta.Entities {
		for name := range def.Faces {
			if face := entity.Face(name); !slices.Contains(faces, face) {
				faces = append(faces, face)
			}
		}
	}
	slices.Sort(faces)
	return faces
}
