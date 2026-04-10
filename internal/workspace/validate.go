package workspace

import (
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// ValidateResult contains the outcome of validating project configuration.
type ValidateResult struct {
	MetamodelValid   bool
	MetamodelError   error
	Metamodel        *metamodel.Metamodel // nil if validation failed
	DataEntryValid   bool
	DataEntryError   error
	DataEntrySkipped bool // true if file doesn't exist
}

// HasErrors returns true if any validation failed.
func (r *ValidateResult) HasErrors() bool {
	return r.MetamodelError != nil || r.DataEntryError != nil
}

// Validate validates project configuration files (metamodel.yaml, data-entry.yaml)
// without loading the full graph. Use this for the `rela validate` command.
// If startDir is empty, it uses the current working directory.
func Validate(startDir string) (*ValidateResult, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	return ValidateWithFS(startDir, fs)
}

// ValidateWithFS validates using the provided filesystem.
func ValidateWithFS(startDir string, fs storage.FS) (*ValidateResult, error) {
	// Discover project
	ctx, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, fmt.Errorf("no project found: run 'rela init' to create one")
	}

	result := &ValidateResult{}

	// Validate metamodel
	mm, _, err := metamodel.Load(ctx.MetamodelPath, fs)
	if err != nil {
		result.MetamodelError = err
	} else {
		result.MetamodelValid = true
		result.Metamodel = mm
	}

	// Validate data-entry.yaml if it exists
	dataEntryPath := filepath.Join(ctx.Root, dataentryconfig.ConfigFile)
	result.DataEntryError = validateDataEntry(dataEntryPath, mm, fs)
	if result.DataEntryError == nil {
		if _, statErr := fs.Stat(dataEntryPath); statErr != nil || mm == nil {
			result.DataEntrySkipped = true
		} else {
			result.DataEntryValid = true
		}
	}

	return result, nil
}

// validateDataEntry validates the data-entry.yaml file.
// Returns nil if file doesn't exist or metamodel is nil (validation skipped).
func validateDataEntry(path string, mm *metamodel.Metamodel, fs storage.FS) error {
	if mm == nil {
		return nil // Can't validate without metamodel
	}

	exists, _ := fileExists(path, fs)
	if !exists {
		return nil
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	return dataentryconfig.ValidateConfig(data, &cfg, mm)
}

func fileExists(path string, fs storage.FS) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	return false, err
}
