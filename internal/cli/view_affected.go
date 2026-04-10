package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/views"
)

var (
	viewAffectedChanged      string
	viewAffectedChangedFiles string
	viewAffectedRoots        string
)

// coverage-ignore: CLI command - tested via integration tests
var viewAffectedCmd = &cobra.Command{
	Use:   "affected <view-name>",
	Short: "Find document roots affected by entity changes",
	Long: `Determines which document root entities are affected by changes to other entities.

Given a set of changed entity IDs (or file paths), this command executes the view
for each root and reports which roots include any of the changed entities.

Use --changed to pass entity IDs directly, or --changed-files to pass file paths
(use - to read from stdin, handy for piping git diff output).

By default, all entities matching the view's entry type are considered as roots.
Use --roots to restrict to specific root entities.

Examples:
  rela view affected document_publish --changed REQ-001,COMP-003
  rela view affected document_publish --changed-files entities/requirement/REQ-001.md
  git diff --name-only HEAD~1 | rela view affected document_publish --changed-files -
  rela view affected document_publish --changed REQ-001 --roots DOC-001,DOC-002`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		viewName := args[0]

		// Load views file
		viewsFile, err := ws.LoadViews()
		if err != nil {
			return fmt.Errorf("failed to load views file: %w", err)
		}

		// Get the view definition
		viewDef, ok := viewsFile.GetView(viewName)
		if !ok {
			return fmt.Errorf("view not found: %s", viewName)
		}

		// Validate the view against the metamodel
		if validationErr := viewDef.Validate(meta, viewName); validationErr != nil {
			return fmt.Errorf("view validation failed: %w", validationErr)
		}

		// Collect changed entity IDs
		changedIDs, err := collectChangedIDs()
		if err != nil {
			return err
		}

		if len(changedIDs) == 0 {
			if viewAffectedChanged == "" && viewAffectedChangedFiles == "" {
				return fmt.Errorf("no changed entities specified: use --changed or --changed-files")
			}
			return fmt.Errorf("no changed entities resolved: check that the specified IDs or file paths match entities in the graph")
		}

		// Determine root entity IDs
		var rootIDs []string
		if viewAffectedRoots != "" {
			rootIDs = strings.Split(viewAffectedRoots, ",")
		} else {
			for _, entity := range ws.EntitiesByType(viewDef.Entry.Type) {
				rootIDs = append(rootIDs, entity.ID)
			}
		}

		// Find affected roots
		engine := views.NewEngine(ws.Snapshot().Graph(), meta)
		affected, err := engine.AffectedRoots(viewDef, changedIDs, rootIDs)
		if err != nil {
			return fmt.Errorf("failed to find affected roots: %w", err)
		}

		for _, entity := range affected {
			fmt.Println(entity.ID)
		}

		return nil
	},
}

// collectChangedIDs gathers entity IDs from --changed and --changed-files flags.
// coverage-ignore: CLI input handling - tested via integration tests
func collectChangedIDs() ([]string, error) {
	seen := make(map[string]bool)
	var ids []string

	addID := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	// From --changed flag
	if viewAffectedChanged != "" {
		for _, id := range strings.Split(viewAffectedChanged, ",") {
			addID(id)
		}
	}

	// From --changed-files flag
	if viewAffectedChangedFiles != "" {
		fileIDs, err := resolveChangedFiles()
		if err != nil {
			return nil, err
		}
		for _, id := range fileIDs {
			addID(id)
		}
	}

	return ids, nil
}

// resolveChangedFiles reads file paths and resolves them to entity IDs.
// Entity file paths map to entity IDs; relation file paths map to both endpoint IDs.
// coverage-ignore: CLI input handling - tested via integration tests
func resolveChangedFiles() ([]string, error) {
	paths, err := readChangedFiles(viewAffectedChangedFiles)
	if err != nil {
		return nil, err
	}

	// Build reverse lookups from file path to entity/relation IDs
	pathToEntityID := make(map[string]string)
	for _, entity := range ws.AllEntities() {
		if entity.FilePath != "" {
			pathToEntityID[entity.FilePath] = entity.ID
		}
	}

	type relationEndpoints struct{ from, to string }
	pathToRelation := make(map[string]relationEndpoints)
	for _, rel := range ws.AllRelations() {
		if rel.FilePath != "" {
			pathToRelation[rel.FilePath] = relationEndpoints{from: rel.From, to: rel.To}
		}
	}

	var ids []string
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		resolved := false
		candidates := []string{path, filepath.Join(ws.Paths().Root, path)}
		for _, candidate := range candidates {
			if id, ok := pathToEntityID[candidate]; ok {
				ids = append(ids, id)
				resolved = true
				break
			}
			if endpoints, ok := pathToRelation[candidate]; ok {
				ids = append(ids, endpoints.from, endpoints.to)
				resolved = true
				break
			}
		}
		if !resolved && verbose {
			fmt.Fprintf(os.Stderr, "warning: file path not resolved to any entity: %s\n", path)
		}
	}

	return ids, nil
}

// readChangedFiles reads file paths from a comma-separated string or from stdin (when value is "-").
// coverage-ignore: CLI input handling
func readChangedFiles(input string) ([]string, error) {
	if input == "-" {
		var paths []string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				paths = append(paths, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		return paths, nil
	}

	return strings.Split(input, ","), nil
}

func init() {
	viewAffectedCmd.Flags().StringVar(&viewAffectedChanged, "changed", "", "Comma-separated changed entity IDs")
	viewAffectedCmd.Flags().StringVar(&viewAffectedChangedFiles, "changed-files", "", "Comma-separated changed file paths, or - for stdin")
	viewAffectedCmd.Flags().StringVar(&viewAffectedRoots, "roots", "", "Comma-separated root entity IDs (default: all entities of entry type)")
}
