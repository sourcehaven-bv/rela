package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// HistoryTagCmd sets, moves or deletes a version tag (TKT-VO6VG9): a name
// that points at one version of an entity, kept beside its history. A
// connector names the version it last synced; a person names a reviewed
// state. The write goes through the entitymanager, so it is authorized
// (`update` on the entity, plus `tag:<ns>` for a name `ns/...`) and audited.
type HistoryTagCmd struct {
	ID      string `arg:"" help:"Entity address: ID, or ID@face for a type with faces."`
	Name    string `arg:"" help:"Tag name, e.g. reviewed or sync/jira. A name 'ns/...' needs the permission tag:ns."`
	Version int    `help:"Tag this 1-based version ordinal (see 'rela history') instead of the current state." default:"0"`
	Expect  string `help:"Tag the current state only while it still matches this version token." default:""`
	Delete  bool   `help:"Delete the tag instead of setting it."`
}

// versionTagWriter is the tag write surface the command calls.
// entitymanager.VersionTags satisfies it.
type versionTagWriter interface {
	TagCurrent(
		ctx context.Context, ref entity.Ref, name store.VersionTagName, expect store.EntityVersion,
		view func(context.Context, entity.Ref) (*entity.Entity, error),
	) (store.VersionMeta, error)
	TagVersion(ctx context.Context, ref entity.Ref, name store.VersionTagName, version int) (store.VersionMeta, error)
	UntagVersion(ctx context.Context, ref entity.Ref, name store.VersionTagName) error
}

// Run dispatches `rela history-tag <address> <name> [--version N] [--expect T] [--delete]`.
func (c *HistoryTagCmd) Run(ctx context.Context, svc *writeServices) error {
	if svc.Versions == nil || svc.VersionTags == nil {
		out.WriteMessage("The active storage backend does not support version tags " +
			"(version tags need the PostgreSQL or SQLite build).")
		return nil
	}
	if err := c.validate(); err != nil {
		return err
	}
	name, err := store.ParseVersionTagName(c.Name)
	if err != nil {
		return err
	}
	ref, err := historyAddress(ctx, svc.Store, svc.Meta, svc.Versions, c.ID)
	if err != nil {
		return err
	}
	if c.Delete {
		return c.untag(ctx, svc.VersionTags, ref, name)
	}
	var meta store.VersionMeta
	if c.Version > 0 {
		meta, err = svc.VersionTags.TagVersion(ctx, ref, name, c.Version)
	} else {
		// The operator shell reads raw, so its token is the stored row's.
		meta, err = svc.VersionTags.TagCurrent(ctx, ref, name, store.EntityVersion(c.Expect), svc.Store.GetEntity)
	}
	var conflict *store.VersionConflictError
	switch {
	case errors.As(err, &conflict):
		return fmt.Errorf("%s changed since token %q; nothing was tagged", ref, c.Expect)
	case isTagVersionMissing(err):
		return fmt.Errorf("no version %d for %s", c.Version, ref)
	case err != nil:
		return fmt.Errorf("tag %s: %w", ref, err)
	}
	out.WriteSuccess("Tagged %s v%d as %q.", ref, meta.Version, name.String())
	return nil
}

func (c *HistoryTagCmd) validate() error {
	if c.Version < 0 {
		return errors.New("--version must be a positive version ordinal")
	}
	if c.Delete && (c.Version > 0 || c.Expect != "") {
		return errors.New("--delete takes no --version or --expect")
	}
	if c.Version > 0 && c.Expect != "" {
		return errors.New("pass --version or --expect, not both")
	}
	return nil
}

func (c *HistoryTagCmd) untag(
	ctx context.Context, w versionTagWriter, ref entity.Ref, name store.VersionTagName,
) error {
	err := w.UntagVersion(ctx, ref, name)
	if isTagVersionMissing(err) {
		return fmt.Errorf("%s has no tag %q", ref, name.String())
	}
	if err != nil {
		return fmt.Errorf("untag %s: %w", ref, err)
	}
	out.WriteSuccess("Deleted tag %q from %s.", name.String(), ref)
	return nil
}

// isTagVersionMissing reports the store's "no such version or tag" answer,
// as distinct from a missing entity, which the manager reports with its own
// error.
func isTagVersionMissing(err error) bool {
	var nf interface{ EntityNotFound() bool }
	return errors.Is(err, store.ErrNotFound) && !errors.As(err, &nf)
}
