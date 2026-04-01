---
id: TKT-KUKH
type: ticket
title: Add dry-run mode for automation testing via EntityRelationStore abstraction
kind: enhancement
priority: medium
effort: m
status: backlog
---

# Add Dry-Run Mode for Automation Testing

Enable testing automations without persisting changes by introducing an `EntityRelationStore` abstraction layer.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Workspace                               │
│  Uses Store interface for all persistence                        │
└─────────────────────────────────┬────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Store interface                           │
│                                                                  │
│  Entity/Relation CRUD          Infrastructure                    │
│  ────────────────────          ──────────────                    │
│  ReadEntity                    Paths, Sync, LoadMetamodel,       │
│  WriteEntity                   LoadViews, SaveCache, etc.        │
│  DeleteEntity                  ReadProjectFile, WriteProjectFile │
│  ListEntities                  Load*Template, Generate*Template  │
│  ReadRelation                                                    │
│  WriteRelation                                                   │
│  DeleteRelation                                                  │
│  ListRelations                                                   │
└─────────────────────────────────┬────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Repository                               │
│                                                                  │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────────┐  │
│  │      ers        │    │     fs      │    │     paths       │  │
│  │ EntityRelation- │    │  storage.FS │    │ project.Context │  │
│  │     Store       │    │             │    │                 │  │
│  └────────┬────────┘    └─────────────┘    └─────────────────┘  │
│           │                                                      │
│  Delegates CRUD            Used for infrastructure methods       │
└───────────┼──────────────────────────────────────────────────────┘
            │
            ▼
┌───────────────────────────────────────────────────────────────────┐
│                   EntityRelationStore interface                    │
│                                                                    │
│  ReadEntity, WriteEntity, DeleteEntity, ListEntities               │
│  ReadRelation, WriteRelation, DeleteRelation, ListRelations        │
└───────────────────────────────────────────────────────────────────┘
            │
            │ implementations
            │
    ┌───────┴───────┬─────────────────┐
    ▼               ▼                 ▼
┌────────┐    ┌──────────┐    ┌─────────────────────────────────┐
│  Disk  │    │   Mem    │    │            Overlay              │
│  ERS   │    │   ERS    │    │                                 │
│        │    │          │    │  base ──▶ DiskERS (reads)       │
│        │    │          │    │  mem  ──▶ MemERS (writes)       │
│        │    │          │    │                                 │
│        │    │          │    │  + Diff() method                │
└────────┘    └──────────┘    └─────────────────────────────────┘
```

## Overlay Behavior

- **Reads**: Check mem first, fall back to base (disk)
- **Writes**: Only go to mem (never touch disk)
- **Deletes**: Mark as deleted in mem, shadow the disk version
- **Diff()**: Returns changes in mem layer (created, updated, deleted)

## Usage

```go
// Dry-run automation
disk := NewDiskERS(fs, paths)
overlay := NewOverlayERS(disk)
repo := NewRepository(overlay, fs, paths)

// Run automations against overlay-backed repo
// ...

// See what would change
changes := overlay.Diff()
for _, e := range changes.Created {
    fmt.Printf("Would create: %s\n", e.ID)
}
```

## Implementation Steps

1. Extract `EntityRelationStore` interface from existing Store
2. Move entity/relation CRUD from Repository to new `DiskERS`
3. Update Repository to delegate CRUD to `ers` field
4. Implement `MemERS` (pure in-memory, maps)
5. Implement `OverlayERS` (layered, with Diff())
6. Add `rela automation test` CLI command

## Benefits

- **Dry-run testing**: Preview automation effects without disk writes
- **Faster tests**: Use MemERS for unit tests (no I/O)
- **Transactions**: Could rollback by discarding overlay
- **Previews**: Show diffs before committing changes
