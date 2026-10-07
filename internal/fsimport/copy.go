package fsimport

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// batchSize is how many rows one target transaction carries. Large enough
// that a sqlite import is not one fsync per row, small enough that a
// transaction never holds a project's worth of rows.
const batchSize = 500

// copier writes the source's records into the target.
type copier struct {
	src *source
	dst Target
	rep *Report

	// imported records what reached the target, for the file
	// reconciliation and the comment copy.
	imported importedSet

	// inPlace is set by [Copy]: no project files are copied, so the
	// .rela configuration files are listed as left where they are.
	inPlace bool
}

// importedSet is what the copy wrote, keyed the way the source files are
// named.
//
// read holds what the source store returned, written or not. A file that
// was read but failed to write already has its error; the reconciliation
// only reports files the store never returned.
type importedSet struct {
	entities  map[string]string // state key (id or id@face) -> type
	families  map[string]string // id -> type
	relations map[string]bool   // relation file stem
	attach    map[string]bool   // entity/property/file

	readEntities  map[string][]string // state key -> types it was read as
	readRelations map[string]bool     // relation file stem

	// keptState and keptMigrations name what the target already held and
	// kept instead of the source's value; verification skips them.
	keptState      map[string]bool
	keptMigrations bool
}

func (c *copier) copyAll(ctx context.Context) error {
	c.imported = importedSet{
		entities:  map[string]string{},
		families:  map[string]string{},
		relations: map[string]bool{},
		attach:    map[string]bool{},

		readEntities:  map[string][]string{},
		readRelations: map[string]bool{},
		keptState:     map[string]bool{},
	}
	steps := []func(context.Context) error{
		c.copyEntities,
		c.copyRelations,
		c.copyAttachments,
		c.copyComments,
		c.copyMigrations,
		c.copyState,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	if err := reconcileFiles(c.src, c.imported, c.rep); err != nil {
		return err
	}
	c.rep.sortSkipped()
	if n := len(c.rep.Errors); n > 0 {
		return fmt.Errorf("%d problem(s) found; nothing was written", n)
	}
	return nil
}

// inBatches runs write over items in target transactions of batchSize.
//
// A row error is recorded by write and does not end the batch: the run fails
// anyway once any error is recorded, and finding every problem in one run is
// the point. Only an error from the transaction itself stops the copy.
func inBatches[T any](ctx context.Context, dst store.Store, items []T,
	write func(context.Context, store.Store, T),
) error {
	for start := 0; start < len(items); start += batchSize {
		batch := items[start:min(start+batchSize, len(items))]
		err := dst.Tx(ctx, func(tx store.Store) error {
			for _, item := range batch {
				write(ctx, tx, item)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("write to the new database: %w", err)
		}
	}
	return nil
}

func (c *copier) copyEntities(ctx context.Context) error {
	var rows []*entity.Entity
	for e, err := range c.src.store.ListEntities(ctx, allEntities()) {
		if err != nil {
			c.rep.fail("read entity: %v", err)
			continue
		}
		ref := entity.FormatStateRef(e.ID, e.Face)
		c.imported.readEntities[ref] = append(c.imported.readEntities[ref], e.Type)
		rows = append(rows, e)
	}
	// fsstore returns every file when one id sits in two type folders.
	// None of them is the entity; the reconciliation reports each file.
	rows = slices.DeleteFunc(rows, func(e *entity.Entity) bool {
		return len(c.imported.readEntities[entity.FormatStateRef(e.ID, e.Face)]) > 1
	})
	return inBatches(ctx, c.dst.Store, rows, func(ctx context.Context, tx store.Store, e *entity.Entity) {
		ref := entity.FormatStateRef(e.ID, e.Face)
		if e.IsLocked() {
			c.rep.fail("entity %s is encrypted", ref)
			return
		}
		out, err := normalizedEntity(c.src, e)
		if err != nil {
			c.rep.fail("entity %s: %v", ref, err)
			return
		}
		if err := tx.CreateEntity(ctx, out.e); err != nil {
			c.rep.fail("entity %s: %v", ref, describeWriteErr(err))
			return
		}
		c.rep.Normalized += out.changed
		c.rep.Entities++
		c.imported.entities[ref] = e.Type
		c.imported.families[e.ID] = e.Type
	})
}

func (c *copier) copyRelations(ctx context.Context) error {
	var rows []*entity.Relation
	for r, err := range c.src.store.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			c.rep.fail("read relation: %v", err)
			continue
		}
		// A file whose frontmatter lacks from, relation or to reads as a
		// relation with an empty part. Its file is reported, with the
		// missing key, by the file reconciliation.
		if r.From == "" || r.Type == "" || r.To == "" {
			continue
		}
		c.imported.readRelations[relationName(r.Identity())] = true
		rows = append(rows, r)
	}
	return inBatches(ctx, c.dst.Store, rows, func(ctx context.Context, tx store.Store, r *entity.Relation) {
		name := relationName(r.Identity())
		if r.IsLocked() {
			c.rep.fail("relation %s is encrypted", name)
			return
		}
		out, err := normalizedRelation(c.src, r)
		if err != nil {
			c.rep.fail("relation %s: %v", name, err)
			return
		}
		data := &store.RelationData{Properties: out.r.Properties, Content: out.r.Content}
		if _, err := tx.CreateRelation(ctx, r.Identity(), data); err != nil {
			c.rep.fail("relation %s: %v", name, describeWriteErr(err))
			return
		}
		c.rep.Normalized += out.changed
		c.rep.Relations++
		c.imported.relations[name] = true
	})
}

func (c *copier) copyAttachments(ctx context.Context) error {
	for _, id := range sortedKeys(c.imported.families) {
		infos, err := c.src.store.ListFamilyAttachments(ctx, id)
		if err != nil {
			c.rep.fail("list attachments of %s: %v", id, err)
			continue
		}
		for _, a := range infos {
			if err := c.copyAttachment(ctx, a); err != nil {
				c.rep.fail("attachment %s: %v", attachmentName(a), describeWriteErr(err))
				continue
			}
			c.rep.Attachments++
			c.imported.attach[attachmentName(a)] = true
		}
	}
	return nil
}

func (c *copier) copyAttachment(ctx context.Context, a store.AttachmentInfo) error {
	rc, err := c.src.store.ReadFamilyAttachment(ctx, a.EntityID, a.Property, a.FileName)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	r, err := lockedReader(rc)
	if err != nil {
		return err
	}
	return c.dst.Store.AttachFamilyFile(ctx, a.EntityID, a.Property, a.FileName, r)
}

// copyComments copies the threads of imported entities. A thread whose
// entity or face was not imported is listed, not copied: the target has
// nothing for it to annotate.
func (c *copier) copyComments(ctx context.Context) error {
	keys, err := c.src.comments.ThreadKeys(ctx)
	if err != nil {
		c.rep.fail("list comment threads: %v", err)
		return nil
	}
	for _, key := range keys {
		path := ".rela/comments/" + key + ".yaml"
		id, face, err := entity.ParseStateRef(key)
		if err != nil {
			c.rep.skip(path, "the file name is not an entity id")
			continue
		}
		typ, ok := c.imported.entities[key]
		if !ok {
			c.rep.skip(path, "comment thread of an entity that was not imported")
			continue
		}
		target := comments.Target{Type: typ, ID: id, Face: face}
		list, err := c.src.comments.List(ctx, target)
		if err != nil {
			c.rep.fail("comments of %s: %v", key, err)
			continue
		}
		for _, cm := range list {
			if err := c.dst.Comments.Add(ctx, target, cm); err != nil {
				c.rep.fail("comment %s on %s: %v", cm.ID, key, err)
				continue
			}
			c.rep.Comments++
		}
	}
	return nil
}

// copyMigrations carries the applied-migration record across whole, so the
// target does not replay migrations the source already ran. A record the
// target already holds is kept: it describes the data already there.
func (c *copier) copyMigrations(ctx context.Context) error {
	st, err := c.src.migState.Load(ctx)
	if err != nil {
		c.rep.fail("read the applied-migration record: %v", err)
		return nil
	}
	if st == nil {
		return nil
	}
	have, err := c.dst.Migrations.Load(ctx)
	if err != nil {
		c.rep.fail("read the database's applied-migration record: %v", err)
		return nil
	}
	if have != nil {
		c.imported.keptMigrations = true
		if !sameJSON(utcState(have), utcState(st)) {
			c.rep.warn("the database already has an applied-migration record; kept it, not the source's")
		}
		return nil
	}
	if err := c.dst.Migrations.Save(ctx, st); err != nil {
		c.rep.fail("write the applied-migration record: %v", err)
	}
	return nil
}

// describeWriteErr adds the cause an operator needs to the store errors whose
// text alone does not say it.
func describeWriteErr(err error) error {
	switch {
	case errors.Is(err, store.ErrConflict):
		return fmt.Errorf("%w (ids are unique regardless of case in the new database, "+
			"so another id differing only in case, or the same id under another type, already holds it)", err)
	case errors.Is(err, errEncrypted):
		return errors.New("is encrypted")
	}
	return err
}

type normalizedEntityRow struct {
	e       *entity.Entity
	changed int
}

func normalizedEntity(src *source, e *entity.Entity) (normalizedEntityRow, error) {
	props, n, err := normalizeProps(e.Properties, src.entityProps(e.Type))
	if err != nil {
		return normalizedEntityRow{}, err
	}
	out := *e
	out.Properties = props
	return normalizedEntityRow{e: &out, changed: n}, nil
}

type normalizedRelationRow struct {
	r       *entity.Relation
	changed int
}

func normalizedRelation(src *source, r *entity.Relation) (normalizedRelationRow, error) {
	props, n, err := normalizeProps(r.Properties, src.relationProps(r.Type))
	if err != nil {
		return normalizedRelationRow{}, err
	}
	out := *r
	out.Properties = props
	return normalizedRelationRow{r: &out, changed: n}, nil
}
