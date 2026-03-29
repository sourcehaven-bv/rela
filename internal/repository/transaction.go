package repository

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Tx represents an active transaction that batches write operations.
// All writes are staged to temporary files (.new suffix) and only
// committed atomically when the transaction completes successfully.
// On error, all staged files are cleaned up automatically.
type Tx interface {
	// WriteEntity stages an entity write. The actual file is written
	// to a temporary location and renamed on commit.
	WriteEntity(entity *model.Entity, meta *metamodel.Metamodel) error

	// WriteRelation stages a relation write.
	WriteRelation(relation *model.Relation) error

	// DeleteEntity marks an entity for deletion. The file is removed on commit.
	DeleteEntity(entityType, id string, meta *metamodel.Metamodel) error

	// DeleteRelation marks a relation for deletion.
	DeleteRelation(from, relType, to string) error
}

// Transaction executes a function within a transaction context.
// All write/delete operations are batched and applied atomically.
// On error, all staged changes are rolled back.
func (r *Repository) Transaction(fn func(tx Tx) error) error {
	tx := &transaction{
		repo:          r,
		stagedWrites:  make(map[string]string), // target -> temp path
		stagedDeletes: make([]string, 0),
	}

	// Execute the user function
	if err := fn(tx); err != nil {
		tx.rollback()
		return err
	}

	// Commit all staged operations
	return tx.commit()
}

// transaction implements Tx with two-phase commit semantics.
type transaction struct {
	repo          *Repository
	stagedWrites  map[string]string // target path -> temp path
	stagedDeletes []string          // paths to delete on commit
}

func (tx *transaction) WriteEntity(entity *model.Entity, meta *metamodel.Metamodel) error {
	filePath := tx.repo.EntityFilePath(entity.Type, entity.ID, meta)
	if filePath == "" {
		return fmt.Errorf("unknown entity type: %s", entity.Type)
	}

	tempPath := filePath + ".new"
	entity.FilePath = filePath

	// Get property order from metamodel if available
	var propertyOrder []string
	if entityDef, ok := meta.GetEntityDef(entity.Type); ok {
		propertyOrder = entityDef.GetPropertyOrder()
	}

	if err := tx.repo.fio.WriteEntity(entity, tempPath, propertyOrder); err != nil {
		return fmt.Errorf("write staged entity: %w", err)
	}

	tx.stagedWrites[filePath] = tempPath
	return nil
}

func (tx *transaction) WriteRelation(relation *model.Relation) error {
	filePath := tx.repo.paths.RelationFilePath(relation.From, relation.Type, relation.To)
	tempPath := filePath + ".new"
	relation.FilePath = filePath

	if err := tx.repo.fio.WriteRelation(relation, tempPath); err != nil {
		return fmt.Errorf("write staged relation: %w", err)
	}

	tx.stagedWrites[filePath] = tempPath
	return nil
}

func (tx *transaction) DeleteEntity(entityType, id string, meta *metamodel.Metamodel) error {
	filePath := tx.repo.EntityFilePath(entityType, id, meta)
	if filePath == "" {
		return fmt.Errorf("unknown entity type: %s", entityType)
	}

	tx.stagedDeletes = append(tx.stagedDeletes, filePath)
	return nil
}

func (tx *transaction) DeleteRelation(from, relType, to string) error {
	filePath := tx.repo.paths.RelationFilePath(from, relType, to)
	tx.stagedDeletes = append(tx.stagedDeletes, filePath)
	return nil
}

// commit applies all staged operations atomically.
// First renames all temp files, then deletes marked files.
func (tx *transaction) commit() error {
	// Phase 1: Rename all staged writes
	renamed := make([]string, 0, len(tx.stagedWrites))
	for target, temp := range tx.stagedWrites {
		if err := tx.repo.fs.Rename(temp, target); err != nil {
			// Rollback already renamed files
			tx.rollbackRenamed(renamed)
			// Clean up remaining temp files
			tx.rollback()
			return fmt.Errorf("commit rename %s: %w", target, err)
		}
		renamed = append(renamed, target)
	}

	// Clear staged writes (they're committed now)
	tx.stagedWrites = make(map[string]string)

	// Phase 2: Delete marked files (best effort, ignore errors)
	for _, path := range tx.stagedDeletes {
		_ = tx.repo.fs.Remove(path)
	}

	return nil
}

// rollback removes all staged temporary files.
func (tx *transaction) rollback() {
	for _, temp := range tx.stagedWrites {
		_ = tx.repo.fs.Remove(temp)
	}
	tx.stagedWrites = make(map[string]string)
}

// rollbackRenamed attempts to restore renamed files.
// This is best-effort since we can't truly undo an atomic rename.
func (tx *transaction) rollbackRenamed(renamed []string) {
	// We can't truly rollback renames, but we can try to clean up
	// by removing the target files that were just created.
	// This leaves the system in an inconsistent state, but that's
	// the nature of two-phase commit without a true transaction log.
	for _, target := range renamed {
		_ = tx.repo.fs.Remove(target)
	}
}

// FindOrphanedTempFiles scans for leftover .new files from interrupted transactions.
// Returns a list of paths to orphaned temp files.
func (r *Repository) FindOrphanedTempFiles() ([]string, error) {
	var orphaned []string

	// Check entities directory
	entityOrphans := r.findTempFilesInDir(r.paths.EntitiesDir)
	orphaned = append(orphaned, entityOrphans...)

	// Check relations directory
	relationOrphans := r.findTempFilesInDir(r.paths.RelationsDir)
	orphaned = append(orphaned, relationOrphans...)

	return orphaned, nil
}

// CleanupOrphanedTempFiles removes all orphaned .new temp files.
func (r *Repository) CleanupOrphanedTempFiles() (int, error) {
	orphaned, err := r.FindOrphanedTempFiles()
	if err != nil {
		return 0, err
	}

	for _, path := range orphaned {
		if removeErr := r.fs.Remove(path); removeErr != nil {
			return 0, fmt.Errorf("remove %s: %w", path, removeErr)
		}
	}

	return len(orphaned), nil
}

func (r *Repository) findTempFilesInDir(dir string) []string {
	var result []string

	entries, err := r.fs.ReadDir(dir)
	if err != nil {
		return nil // Directory might not exist, that's fine
	}

	for _, entry := range entries {
		name := entry.Name()
		path := dir + "/" + name

		if entry.IsDir() {
			// Recurse into subdirectories (e.g., entities/tasks/)
			result = append(result, r.findTempFilesInDir(path)...)
		} else if len(name) > 4 && name[len(name)-4:] == ".new" {
			result = append(result, path)
		}
	}

	return result
}
