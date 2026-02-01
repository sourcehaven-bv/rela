package migration

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// DetectionResult holds information about a detected migration need.
type DetectionResult struct {
	Migration   Migration
	Description string
	Count       int // Number of occurrences found (for informational purposes)
}

// Result holds information about an applied migration.
type Result struct {
	Migration Migration
	Applied   bool
	Error     error
}

// FileResult holds the results of migrating a single file.
type FileResult struct {
	Path       string
	FileType   FileType
	Detections []DetectionResult
	Results    []Result
	Error      error
}

// Detect checks a YAML file for migrations that need to be applied
// using the given filesystem.
func Detect(path string, ft FileType, fs storage.FS) ([]DetectionResult, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return DetectFromNode(&doc, ft), nil
}

// DetectFromNode checks a parsed YAML document for migrations.
func DetectFromNode(doc *yaml.Node, ft FileType) []DetectionResult {
	var results []DetectionResult
	migrations := ForFileType(ft)

	for _, m := range migrations {
		if m.Detect(doc) {
			results = append(results, DetectionResult{
				Migration:   m,
				Description: m.Description(),
			})
		}
	}

	return results
}

// Apply runs all applicable migrations on a file using the given filesystem.
// Returns results for each migration attempted.
func Apply(path string, ft FileType, fs storage.FS) (*FileResult, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	result := &FileResult{
		Path:     path,
		FileType: ft,
	}

	// First detect what needs migration
	result.Detections = DetectFromNode(&doc, ft)

	if len(result.Detections) == 0 {
		return result, nil
	}

	// Apply each detected migration
	for _, detection := range result.Detections {
		mr := Result{Migration: detection.Migration}

		if err := detection.Migration.Apply(&doc); err != nil {
			mr.Error = err
		} else {
			mr.Applied = true
		}

		result.Results = append(result.Results, mr)
	}

	// Check if any migrations failed - store error in result but don't return it as function error
	for _, mr := range result.Results {
		if mr.Error != nil {
			result.Error = fmt.Errorf("migration %s failed: %w", mr.Migration.Name(), mr.Error)
			return result, result.Error
		}
	}

	// Write the migrated file back
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		result.Error = fmt.Errorf("encoding YAML: %w", err)
		return result, nil
	}
	if err := encoder.Close(); err != nil {
		result.Error = fmt.Errorf("closing encoder: %w", err)
		return result, nil
	}

	if err := fs.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		result.Error = fmt.Errorf("writing file: %w", err)
		return result, nil
	}

	return result, nil
}

// CheckOnly runs detection without applying changes using the given filesystem.
// Useful for CI or pre-flight checks.
func CheckOnly(path string, ft FileType, fs storage.FS) (*FileResult, error) {
	detections, err := Detect(path, ft, fs)
	if err != nil {
		return nil, err
	}

	return &FileResult{
		Path:       path,
		FileType:   ft,
		Detections: detections,
	}, nil
}

// NeedsMigration returns true if any migrations were detected.
func (r *FileResult) NeedsMigration() bool {
	return len(r.Detections) > 0
}

// MigrationsApplied returns the count of successfully applied migrations.
func (r *FileResult) MigrationsApplied() int {
	count := 0
	for _, mr := range r.Results {
		if mr.Applied {
			count++
		}
	}
	return count
}

// HasErrors returns true if any migrations failed.
func (r *FileResult) HasErrors() bool {
	return r.Error != nil
}

// Error represents an error that includes suggestions for migration.
type Error struct {
	FilePath   string
	Detections []DetectionResult
}

func (e *Error) Error() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s uses deprecated syntax:\n", e.FilePath)
	for _, d := range e.Detections {
		fmt.Fprintf(&buf, "  - %s\n", d.Description)
	}
	buf.WriteString("\nRun 'rela migrate' to update your project files.")
	return buf.String()
}

// IsMigrationError checks if an error is a migration Error.
func IsMigrationError(err error) bool {
	var migErr *Error
	return errors.As(err, &migErr)
}
