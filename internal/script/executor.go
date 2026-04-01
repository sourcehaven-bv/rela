// Package script orchestrates script execution for automations.
// It combines lua.Runtime with workspace operations, handling:
// path validation, secure file loading, and entity context injection.
//
// This package bridges lua (runtime) and workspace (domain) without either
// depending on the other - each defines the interface it needs, script glues them.
package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// scriptsDir is the directory where script files must be located.
const scriptsDir = "scripts"

// Executor runs scripts with entity context. It centralizes all script
// execution concerns: path validation, secure file loading, sandbox
// enforcement, and entity context setup.
//
// Timeout is handled by lua.Runtime (default 30s, configurable via lua.WithTimeout).
type Executor struct {
	ws          lua.WorkspaceInterface
	meta        *metamodel.Metamodel
	projectRoot string
}

// New creates a script executor.
func New(ws lua.WorkspaceInterface, meta *metamodel.Metamodel, projectRoot string) *Executor {
	return &Executor{
		ws:          ws,
		meta:        meta,
		projectRoot: projectRoot,
	}
}

// ExecuteCode runs inline script code with the given entity context.
func (e *Executor) ExecuteCode(code string, entity, oldEntity *model.Entity) error {
	return e.execute(code, entity, oldEntity)
}

// ExecuteFile loads and runs a script file from the scripts/ directory.
// The path must be a local path (no ".." or absolute paths) with .lua extension.
func (e *Executor) ExecuteFile(path string, entity, oldEntity *model.Entity) error {
	code, err := e.loadScript(path)
	if err != nil {
		return err
	}
	return e.execute(code, entity, oldEntity)
}

// execute runs Lua code with entity context.
// Timeout is handled by lua.Runtime (default 30s).
func (e *Executor) execute(code string, entity, oldEntity *model.Entity) error {
	var output bytes.Buffer
	runtime := lua.New(e.ws, e.meta, e.projectRoot, &output)
	defer runtime.Close()

	// Set entity context as Lua globals
	ls := runtime.LState()
	if entity != nil {
		ls.SetGlobal("entity", lua.EntityToTable(ls, entity))
	}
	if oldEntity != nil {
		ls.SetGlobal("old_entity", lua.EntityToTable(ls, oldEntity))
	}

	return runtime.RunString(code)
}

// loadScript loads a script from the scripts/ directory using os.OpenRoot
// for traversal-resistant file access.
func (e *Executor) loadScript(scriptPath string) (string, error) {
	// Security: Validate path is local (no "..", no absolute paths)
	if !filepath.IsLocal(scriptPath) {
		return "", fmt.Errorf(
			"script path must be a local path (no '..' or absolute paths): %s", scriptPath)
	}

	// Security: Must have .lua extension
	if !strings.HasSuffix(scriptPath, ".lua") {
		return "", fmt.Errorf("script must have .lua extension: %s", scriptPath)
	}

	// Use os.OpenRoot for traversal-resistant access.
	// Error messages intentionally omit system paths to prevent information leakage.
	root, err := os.OpenRoot(e.projectRoot)
	if err != nil {
		return "", errors.New("cannot access project directory")
	}
	defer root.Close()

	scriptsRoot, err := root.OpenRoot(scriptsDir)
	if err != nil {
		return "", errors.New("cannot access scripts directory")
	}
	defer scriptsRoot.Close()

	scriptFile, err := scriptsRoot.Open(scriptPath)
	if err != nil {
		return "", fmt.Errorf("script not found: %s (must be in scripts/ directory)", scriptPath)
	}
	defer scriptFile.Close()

	content, err := io.ReadAll(scriptFile)
	if err != nil {
		return "", fmt.Errorf("cannot read script: %s", scriptPath)
	}

	return string(content), nil
}
