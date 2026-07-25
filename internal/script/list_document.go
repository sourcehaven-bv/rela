package script

import (
	"context"
	"io"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// ExecuteListDocument loads and runs a Lua script in LIST document-rendering
// mode — the `lists.<id>.export_render` override. The script's captured
// stdout is the rendered markdown, which the data-entry layer feeds to a
// format transform.
//
// documentID is the synthetic config identity ("export:list:<listID>");
// lrc carries the list id, the lazy row provider, and the read-only resolved
// query context. The rows are the ones the CALLER already resolved through
// the ACL read path, filtered, sorted, and capped — the script renders those
// and does not derive its own set. That keeps an export showing exactly what
// the on-screen view showed, and keeps the caller's row cap meaningful.
//
// Like [Engine.ExecuteDocument] and [Engine.ExecuteAction] this is a typed
// seam — intentionally NOT taking variadic lua.Option — so callers cannot
// inject arbitrary opts (e.g. forge WithOutputDir or WithActionMode).
func (e *Engine) ExecuteListDocument(
	ctx context.Context,
	path string,
	deps lua.WriteDeps,
	stdout io.Writer,
	documentID string,
	lrc lua.ListRenderContext,
	timeout time.Duration,
) error {
	// The list id stands in for the entity id in the error envelope's subject
	// slot: a list render has no entry entity, and "which list" is the useful
	// thing to see in a failure.
	return e.runDocumentScript(ctx, path, deps, stdout,
		lua.WithListDocumentMode(documentID, lrc), lrc.ListID, timeout)
}

// runDocumentScript is the shared body behind the document-mode entry points
// (ExecuteDocument, ExecuteListDocument): load the script, build the standard
// opt set, run it with the file path as chunkname, and shape any failure into
// a *lua.ScriptError.
//
// modeOpt is what distinguishes the callers, and keeping it a PARAMETER rather
// than exposing variadic lua.Option is what preserves the typed seam: the
// public methods each supply exactly one mode, so no caller can forge
// WithOutputDir or WithActionMode. subject fills the error envelope's subject
// slot (an entity id, or a list id where there is no entry entity).
func (e *Engine) runDocumentScript(
	ctx context.Context,
	path string,
	deps lua.WriteDeps,
	stdout io.Writer,
	modeOpt lua.Option,
	subject string,
	timeout time.Duration,
) error {
	scriptCode, err := loadScript(deps.ProjectRoot, path)
	if err != nil {
		return err
	}

	// ctx + principal are threaded so the render runs under the CALLER's
	// identity: its reads are ACL-bound (TKT-ZF2DTV) and it cancels with the
	// request.
	opts := []lua.Option{
		modeOpt,
		lua.WithCache(e.cache),
		lua.WithContext(ctx),
		lua.WithPrincipal(principal.From(ctx)),
	}
	if timeout > 0 {
		opts = append(opts, lua.WithTimeout(timeout))
	}

	runtime, err := NewWriterRuntime(deps, path, stdout, opts...)
	if err != nil {
		return err
	}
	defer runtime.Close()

	// RunFileContent (not RunString) so gopher-lua receives the script path
	// as the chunkname — that lands in the message handler's frame captures
	// and lets ScriptError.Source populate from the right file.
	//nolint:contextcheck // ctx threaded via WithContext above
	if runErr := runtime.RunFileContent(path, []byte(scriptCode), nil); runErr != nil {
		return wrapScriptError(lua.SurfaceDocument, scriptsDir, path, subject,
			runtime.ErrorFrames(), nil, runErr, deps.ProjectRoot)
	}
	return nil
}
