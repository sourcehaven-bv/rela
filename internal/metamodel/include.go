package metamodel

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// partialMetamodel holds the parsed content of an included file
type partialMetamodel struct {
	sourcePath string // relative path for error messages

	Types       map[string]CustomType  `yaml:"types"`
	Entities    map[string]EntityDef   `yaml:"entities"`
	Relations   map[string]RelationDef `yaml:"relations"`
	Validations []ValidationRule       `yaml:"validations"`
	Includes    []string               `yaml:"includes"`

	// Fields that are not allowed in included files
	Version   string `yaml:"version"`
	Namespace string `yaml:"namespace"`
}

// includeState tracks file processing state during recursive include resolution.
// It distinguishes between files currently being processed (in the call stack,
// indicating a circular include) and files that have been fully processed
// (allowing diamond includes where the same file is included from multiple paths).
type includeState struct {
	inStack   map[string]bool // files currently in the recursion stack (circular detection)
	processed map[string]bool // files already fully resolved (diamond skip)
}

// loadWithIncludes loads a metamodel and recursively resolves all includes.
// rootDir is the project root directory (where the root metamodel.yaml lives).
// All include paths are resolved relative to rootDir.
//
// Returns the absolute paths of all include files that were read (not including
// the root metamodel path itself).
func loadWithIncludes(root *Metamodel, rootPath, rootDir string, fs storage.FS) ([]string, error) {
	if len(root.Includes) == 0 {
		return nil, nil
	}

	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	state := &includeState{
		inStack:   map[string]bool{absRoot: true},
		processed: map[string]bool{},
	}

	// Collect all partials in depth-first order
	var partials []*partialMetamodel
	for _, inc := range root.Includes {
		collected, err := resolveIncludes(rootDir, inc, rootPath, state, []string{rootPath}, fs)
		if err != nil {
			return nil, err
		}
		partials = append(partials, collected...)
	}

	// Merge all partials into the root metamodel
	if err := mergeIncludes(root, rootPath, partials); err != nil {
		return nil, err
	}

	// Rebuild the alias map to include entities from all files
	root.rebuildAliasMap()

	// Collect resolved include paths from the processed set (excludes root)
	includePaths := make([]string, 0, len(state.processed))
	for absPath := range state.processed {
		includePaths = append(includePaths, absPath)
	}

	return includePaths, nil
}

// resolveIncludes recursively resolves an include file and all its nested includes.
// Returns a flat list of partials in depth-first order.
func resolveIncludes(
	rootDir, includePath, includedFrom string,
	state *includeState, chain []string, fs storage.FS,
) ([]*partialMetamodel, error) {
	// Resolve path relative to the project root
	fullPath := filepath.Join(rootDir, includePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, err
	}

	// Check for circular includes (file is in the current recursion stack)
	if state.inStack[absPath] {
		return nil, &CircularIncludeError{
			Chain: append(chain, includePath),
		}
	}

	// Diamond include: file was already fully processed from another path — skip
	if state.processed[absPath] {
		return nil, nil
	}

	// Check file exists
	data, err := fs.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &IncludeNotFoundError{
				Path:         includePath,
				IncludedFrom: includedFrom,
			}
		}
		return nil, err
	}

	// Parse the partial metamodel
	var partial partialMetamodel
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return nil, err
	}
	partial.sourcePath = includePath

	// Validate no root-only fields
	if partial.Version != "" {
		return nil, &IncludeHasRootFieldError{Path: includePath, Field: "version"}
	}
	if partial.Namespace != "" {
		return nil, &IncludeHasRootFieldError{Path: includePath, Field: "namespace"}
	}

	// Push onto stack for circular detection
	state.inStack[absPath] = true

	// Recursively resolve nested includes (depth-first)
	var partials []*partialMetamodel
	newChain := append(chain, includePath) //nolint:gocritic // intentional append to new slice
	for _, nestedInc := range partial.Includes {
		collected, err := resolveIncludes(rootDir, nestedInc, includePath, state, newChain, fs)
		if err != nil {
			return nil, err
		}
		partials = append(partials, collected...)
	}

	// Pop from stack, mark as fully processed
	delete(state.inStack, absPath)
	state.processed[absPath] = true

	// Add this partial after its dependencies (depth-first)
	partials = append(partials, &partial)

	return partials, nil
}

// mergeIncludes merges all partial metamodels into the root metamodel.
// Returns an error if any duplicate definitions are found.
func mergeIncludes(root *Metamodel, rootPath string, partials []*partialMetamodel) error {
	// Track which file defined each name (for duplicate detection)
	typeOwner := make(map[string]string)
	entityOwner := make(map[string]string)
	relationOwner := make(map[string]string)
	validationOwner := make(map[string]string)

	// Register root's existing definitions
	for name := range root.Types {
		typeOwner[name] = rootPath
	}
	for name := range root.Entities {
		entityOwner[name] = rootPath
	}
	for name := range root.Relations {
		relationOwner[name] = rootPath
	}
	for _, v := range root.Validations {
		validationOwner[v.Name] = rootPath
	}

	// Initialize maps if nil (root may not define any)
	if root.Types == nil {
		root.Types = make(map[string]CustomType)
	}
	if root.Entities == nil {
		root.Entities = make(map[string]EntityDef)
	}
	if root.Relations == nil {
		root.Relations = make(map[string]RelationDef)
	}

	// Merge each partial
	for _, p := range partials {
		// Merge types
		for name, def := range p.Types {
			if owner, exists := typeOwner[name]; exists {
				return &DuplicateDefinitionError{
					Kind: "type", Name: name,
					File1: owner, File2: p.sourcePath,
				}
			}
			typeOwner[name] = p.sourcePath
			root.Types[name] = def
		}

		// Merge entities
		for name, def := range p.Entities {
			if owner, exists := entityOwner[name]; exists {
				return &DuplicateDefinitionError{
					Kind: "entity", Name: name,
					File1: owner, File2: p.sourcePath,
				}
			}
			entityOwner[name] = p.sourcePath
			root.Entities[name] = def
		}

		// Merge relations
		for name, def := range p.Relations {
			if owner, exists := relationOwner[name]; exists {
				return &DuplicateDefinitionError{
					Kind: "relation", Name: name,
					File1: owner, File2: p.sourcePath,
				}
			}
			relationOwner[name] = p.sourcePath
			root.Relations[name] = def
		}

		// Merge validations (check name uniqueness)
		for _, v := range p.Validations {
			if owner, exists := validationOwner[v.Name]; exists {
				return &DuplicateDefinitionError{
					Kind: "validation", Name: v.Name,
					File1: owner, File2: p.sourcePath,
				}
			}
			validationOwner[v.Name] = p.sourcePath
			root.Validations = append(root.Validations, v)
		}
	}

	// Clear includes from the final result (they've been resolved)
	root.Includes = nil

	return nil
}
