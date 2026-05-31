package cli

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/importer"
)

// ImportCmd imports entities and relations from JSON, YAML, or CSV.
type ImportCmd struct {
	File          string `arg:"" help:"Input file path."`
	Format        string `short:"f" help:"Input format (json, yaml, csv). Auto-detected from extension if not specified."`
	DryRun        bool   `short:"n" name:"dry-run" help:"Validate without creating files."`
	Update        bool   `short:"u" help:"Replace existing entities instead of failing on duplicates (full replacement, not merge)."`
	SkipErrors    bool   `name:"skip-errors" help:"Continue importing on validation errors."`
	RelationsFile string `short:"r" name:"relations" help:"Path to relations CSV file (for CSV imports)."`
}

// Run dispatches `rela import <file>`.
func (c *ImportCmd) Run(svc *cliServices) error {
	opts := importer.Options{
		Format:        importer.Format(c.Format),
		DryRun:        c.DryRun,
		Update:        c.Update,
		SkipErrors:    c.SkipErrors,
		RelationsFile: c.RelationsFile,
	}
	imp := importer.New(svc.Store(), svc.Meta(), opts, importer.NewImportSource(svc.FS()))

	if c.DryRun {
		out.WriteInfo("Dry run - validating without creating files...")
	}
	result, err := imp.ImportFile(c.File)
	if err != nil {
		return err
	}

	if c.DryRun {
		reportImportDryRun(result)
	} else {
		reportImportApplied(result)
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		out.WriteWarning("Errors encountered:")
		for _, e := range result.Errors {
			out.WriteError("  %s", e.Error())
		}
	}
	if len(result.Errors) > 0 && !c.SkipErrors {
		return fmt.Errorf("import completed with %d errors", len(result.Errors))
	}
	return nil
}

func reportImportDryRun(result *importer.Result) {
	out.WriteInfo("Validation complete:")
	if result.EntitiesCreated > 0 {
		out.WriteInfo("  Would create %d entities", result.EntitiesCreated)
	}
	if result.RelationsCreated > 0 {
		out.WriteInfo("  Would create %d relations", result.RelationsCreated)
	}
	if result.EntitiesSkipped > 0 {
		out.WriteWarning("  Would skip %d entities (errors)", result.EntitiesSkipped)
	}
	if result.RelationsSkipped > 0 {
		out.WriteWarning("  Would skip %d relations (errors)", result.RelationsSkipped)
	}
}

func reportImportApplied(result *importer.Result) {
	if result.EntitiesCreated > 0 {
		out.WriteSuccess("Created %d entities", result.EntitiesCreated)
	}
	if result.EntitiesUpdated > 0 {
		out.WriteSuccess("Updated %d entities", result.EntitiesUpdated)
	}
	if result.RelationsCreated > 0 {
		out.WriteSuccess("Created %d relations", result.RelationsCreated)
	}
	if result.EntitiesSkipped > 0 {
		out.WriteWarning("Skipped %d entities (errors)", result.EntitiesSkipped)
	}
	if result.RelationsSkipped > 0 {
		out.WriteWarning("Skipped %d relations (errors or duplicates)", result.RelationsSkipped)
	}
}
