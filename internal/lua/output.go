// Lua bindings for script output: rela.output, rela.write_file, and the
// document/action-mode print override.
package lua

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// defaultOutputDir is the default directory where Lua scripts can write files.
const defaultOutputDir = "output"

// outputBindings implements the output-and-filesystem bindings: rela.output,
// rela.write_file, and the print override for captured-stdout modes. A type of
// its own rather than more methods on [Runtime] (the urlHelpers rationale in
// urls.go): the bindings need exactly these five values from the runtime, all
// of which are fixed once the constructor's options have been applied —
// newRuntime builds this AFTER the option loop, so plain value fields suffice
// (unlike cacheBindings' scriptPath, which is mutable per run and needs a
// closure). Registration stays on Runtime: rela.output in
// registerReadBindings, rela.write_file (capability-gated) in
// registerWriteBindings, print in newRuntime.
type outputBindings struct {
	stdout      io.Writer
	outputDir   string // Directory where write_file can write (defaults to "output")
	projectRoot string // Anchors a relative outputDir
	isAction    bool   // true when running as an action (changes rela.output behavior)
	isDocument  bool   // document mode: rela.output becomes a warning
}

// luaPrint replaces gopher-lua's base print so its output lands in
// b.stdout rather than os.Stdout. Matches Lua's stock print: each
// argument is stringified via __tostring, joined with tabs, terminated
// by a newline.
func (b *outputBindings) luaPrint(ls *lua.LState) int {
	top := ls.GetTop()
	for i := 1; i <= top; i++ {
		if i > 1 {
			fmt.Fprint(b.stdout, "\t")
		}
		fmt.Fprint(b.stdout, ls.ToStringMeta(ls.Get(i)).String())
	}
	fmt.Fprintln(b.stdout)
	return 0
}

// luaOutput implements rela.output(data) — JSON-encode data to stdout, except
// in action/document modes where it degrades to a warning line.
func (b *outputBindings) luaOutput(ls *lua.LState) int {
	// Type-check the arg up front; the Lua → Go conversion is deferred
	// past the mode guards so muted modes (action/document) don't pay
	// for converting a potentially-large nested table.
	data := ls.CheckAny(1)

	if b.isAction {
		// In action mode, rela.output is a no-op. Log a warning so script
		// authors notice that output should use the return statement instead.
		fmt.Fprintln(b.stdout, "warning: rela.output() called in action mode; use 'return' to produce the response")
		return 0
	}

	if b.isDocument {
		// In document mode, captured stdout is the rendered document.
		// Raw JSON in the middle of rendered markdown is almost always a
		// mistake — emit a warning line (visible in the panel) so the
		// script author notices, rather than silently producing garbage.
		fmt.Fprintln(b.stdout, "warning: rela.output() called in document mode; use print() to emit markdown")
		return 0
	}

	goData := luaValueToGo(data)
	encoder := json.NewEncoder(b.stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(goData); err != nil {
		ls.RaiseError("JSON encoding error: %s", err.Error())
		return 0
	}
	return 0
}

// luaWriteFile implements rela.write_file(path, content, opts?)
// Files can ONLY be written to the configured output directory for security.
// Path is relative to output dir (e.g., "report.txt" -> "{output}/report.txt").
// Options:
//   - ensure_newline: boolean - ensure content ends with a newline (default: false)
func (b *outputBindings) luaWriteFile(ls *lua.LState) int {
	path := ls.CheckString(1)
	content := ls.CheckString(2)

	if path == "" {
		ls.RaiseError("write_file: path cannot be empty")
		return 0
	}

	// Parse options if provided
	ensureNewline := false
	if ls.GetTop() >= 3 && ls.Get(3).Type() == lua.LTTable {
		opts := ls.CheckTable(3)
		if v := opts.RawGetString("ensure_newline"); v != lua.LNil {
			if bo, ok := v.(lua.LBool); ok {
				ensureNewline = bool(bo)
			}
		}
	}

	// Ensure content ends with newline if requested
	if ensureNewline && content != "" && content[len(content)-1] != '\n' {
		content += "\n"
	}

	// Validate the path is local (no "..", no absolute paths)
	if !filepath.IsLocal(path) {
		ls.RaiseError("write_file: path must be a local path (no '..' or absolute paths)")
		return 0
	}

	// Build the full path within output directory
	var outputPath string
	if filepath.IsAbs(b.outputDir) {
		outputPath = b.outputDir
	} else {
		outputPath = filepath.Join(b.projectRoot, b.outputDir)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		ls.RaiseError("write_file: cannot create output directory: %s", err.Error())
		return 0
	}

	// Ensure parent directories within output/ exist
	fullPath := filepath.Join(outputPath, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		ls.RaiseError("write_file: cannot create directory: %s", err.Error())
		return 0
	}

	// Write the file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		ls.RaiseError("write_file: cannot write file: %s", err.Error())
		return 0
	}

	return 0
}
