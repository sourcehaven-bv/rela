package script

import (
	"bytes"
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// JobInvocation carries the per-call inputs of one flow job attempt
// (TKT-PY2G11W).
type JobInvocation struct {
	// Subject is the run's subject as the job's identity may read it, as
	// the `entity` global. The caller reads it under that identity's ACL;
	// this package does no ACL of its own.
	//
	// Nil: accepted, leaves `entity` unset.
	Subject *entity.Entity

	// Params are rela.params: the attempt's keys, so the script can drop a
	// repeat.
	Params map[string]string

	// Capabilities is the job's `capabilities:` grant. The zero value
	// grants nothing.
	Capabilities lua.Capabilities

	// Timeout bounds the run. The attempt's deadline on ctx bounds it too.
	Timeout time.Duration
}

// ExecuteJob runs the script at path under scripts/ as one attempt of a flow
// job step. The identity is the ctx principal, which the caller stamps and
// binds. A failure is a *lua.ScriptError; its Permanent field is set when the
// script raised a table with `retry = false`.
func (e *Engine) ExecuteJob(ctx context.Context, path string, deps lua.WriteDeps, inv JobInvocation) error {
	code, err := deps.ReadScript(ctx, scriptsDir, path)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	runtime, err := NewWriterRuntime(deps, path, &output,
		lua.WithCache(e.cache), lua.WithContext(ctx),
		lua.WithCapabilities(inv.Capabilities),
		lua.WithParams(inv.Params),
		lua.WithTimeout(inv.Timeout),
		// Flow job scripts are operator-authored files under scripts/.
		lua.WithExternalRefWrites(),
		lua.WithPrincipal(principal.From(ctx)))
	if err != nil {
		return err
	}
	defer runtime.Close()
	runtime.SetScriptPath(path)
	entityID := ""
	if inv.Subject != nil {
		ls := runtime.LState()
		ls.SetGlobal("entity", lua.EntityToTable(ls, inv.Subject))
		entityID = inv.Subject.ID
	}
	//nolint:contextcheck // ctx threaded via WithContext, as in execute
	if runErr := runtime.RunChunk(code, projectPath(scriptsDir, path)); runErr != nil {
		return wrapScriptError(lua.SurfaceJob, scriptsDir, path, entityID,
			runtime.ErrorFrames(), output.Bytes(), runErr, deps.SourceFS(ctx))
	}
	return nil
}
