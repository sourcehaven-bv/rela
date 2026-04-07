// Lua bindings for the ai.* module.
//
// DELIBERATE CONVENTION DEVIATION:
//
// All other rela Lua bindings raise errors via ls.RaiseError. The ai.*
// bindings instead return (nil, err_table) for *expected runtime
// failures* — network errors, HTTP errors, missing config, rate limits,
// upstream 5xx, etc. — because AI calls are inherently network-bound and
// scripts should be able to handle failure inline rather than wrap every
// call in pcall.
//
// PROGRAMMING ERRORS still raise via RaiseError (wrong argument type,
// empty messages list, malformed messages entry). The taxonomy is:
//
//	expected runtime failure  -> (nil, err_table)
//	programming error         -> RaiseError
//
// The error table has stable fields: kind (string), status (number),
// message (string), retry_after (number, seconds). Scripts branch on
// err.kind (e.g. "rate_limited", "auth", "server_error", ...) without
// parsing prose error messages.
//
// CONCURRENCY: ai.chat assumes single-threaded LState use. gopher-lua
// *lua.LState is NOT safe for concurrent goroutine use. ai.Provider
// implementations must be safe to share across runtimes (the default
// OpenAICompatProvider is, because http.Client is).
//
// Do not "fix" this convention without reading the planning document
// for TKT-YBKB. The string-error alternative was deliberately rejected.
package lua

import (
	"context"
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/ai"
)

// registerAIModule installs the top-level `ai` global with `chat` and
// `complete` functions. The global is registered unconditionally; if no
// provider is wired into the Runtime, the functions return a typed
// not_configured error so scripts can feature-detect uniformly.
func (r *Runtime) registerAIModule() {
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "chat", r.L.NewFunction(r.luaAIChat))
	r.L.SetField(tbl, "complete", r.L.NewFunction(r.luaAIComplete))
	r.L.SetGlobal("ai", tbl)
}

// luaAIChat implements ai.chat({messages, model?, temperature?, max_tokens?})
// -> (result_table, nil) on success, (nil, err_table) on failure.
func (r *Runtime) luaAIChat(ls *lua.LState) int {
	if r.aiProvider == nil {
		return pushAIError(ls, &ai.Error{
			Kind:    ai.ErrNotConfigured,
			Message: "AI is not configured: create .rela/ai.yaml with base_url and model",
		})
	}

	opts := ls.CheckTable(1)

	req, parseErr := parseChatRequest(opts)
	if parseErr != nil {
		ls.RaiseError("ai.chat: %s", parseErr.Error())
		return 0
	}

	resp, err := r.aiProvider.Chat(chatContext(r), req)
	if err != nil {
		var aiErr *ai.Error
		if !errors.As(err, &aiErr) {
			// Should not happen — every ai package error is *ai.Error.
			// Treat as a programming bug and raise loudly.
			ls.RaiseError("ai.chat: unexpected error type %T: %v", err, err)
			return 0
		}
		return pushAIError(ls, aiErr)
	}

	ls.Push(chatResponseToTable(ls, resp))
	ls.Push(lua.LNil)
	return 2
}

// luaAIComplete implements ai.complete(prompt) -> (string, nil) or
// (nil, err_table). Convenience wrapper around ai.chat for the common
// single-user-message case.
func (r *Runtime) luaAIComplete(ls *lua.LState) int {
	if r.aiProvider == nil {
		return pushAIError(ls, &ai.Error{
			Kind:    ai.ErrNotConfigured,
			Message: "AI is not configured: create .rela/ai.yaml with base_url and model",
		})
	}

	prompt := ls.CheckString(1)

	resp, err := r.aiProvider.Chat(chatContext(r), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		var aiErr *ai.Error
		if !errors.As(err, &aiErr) {
			ls.RaiseError("ai.complete: unexpected error type %T: %v", err, err)
			return 0
		}
		return pushAIError(ls, aiErr)
	}

	ls.Push(lua.LString(resp.Content))
	ls.Push(lua.LNil)
	return 2
}

// chatContext returns the context to use for AI calls. If the runtime
// has a Lua-state context (set by applyTimeout), use that so timeouts
// propagate; otherwise fall back to context.Background().
func chatContext(r *Runtime) context.Context {
	if ctx := r.L.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// parseChatRequest converts a Lua options table into an ai.ChatRequest.
// Returns an error for programming mistakes (wrong types, missing
// messages, etc.) — callers raise these as Lua errors.
func parseChatRequest(opts *lua.LTable) (ai.ChatRequest, error) {
	var req ai.ChatRequest

	// messages (required)
	messagesVal := opts.RawGetString("messages")
	messagesTbl, ok := messagesVal.(*lua.LTable)
	if !ok {
		return req, errors.New("messages must be a table")
	}
	count := messagesTbl.Len()
	if count == 0 {
		return req, errors.New("messages must not be empty")
	}
	req.Messages = make([]ai.Message, 0, count)
	for i := 1; i <= count; i++ {
		entryVal := messagesTbl.RawGetInt(i)
		entryTbl, ok := entryVal.(*lua.LTable)
		if !ok {
			return req, fmt.Errorf("messages[%d] must be a table", i)
		}
		role, ok := entryTbl.RawGetString("role").(lua.LString)
		if !ok || role == "" {
			return req, fmt.Errorf("messages[%d].role must be a non-empty string", i)
		}
		content, ok := entryTbl.RawGetString("content").(lua.LString)
		if !ok {
			return req, fmt.Errorf("messages[%d].content must be a string", i)
		}
		req.Messages = append(req.Messages, ai.Message{Role: string(role), Content: string(content)})
	}

	// model (optional)
	if v := opts.RawGetString("model"); v != lua.LNil {
		s, ok := v.(lua.LString)
		if !ok {
			return req, errors.New("model must be a string")
		}
		req.Model = string(s)
	}

	// temperature (optional)
	if tempVal := opts.RawGetString("temperature"); tempVal != lua.LNil {
		num, ok := tempVal.(lua.LNumber)
		if !ok {
			return req, errors.New("temperature must be a number")
		}
		t := float64(num)
		req.Temperature = &t
	}

	// max_tokens (optional)
	if maxVal := opts.RawGetString("max_tokens"); maxVal != lua.LNil {
		num, ok := maxVal.(lua.LNumber)
		if !ok {
			return req, errors.New("max_tokens must be a number")
		}
		mt := int(num)
		req.MaxTokens = &mt
	}

	return req, nil
}

// chatResponseToTable converts a *ai.ChatResponse to a flat Lua table.
func chatResponseToTable(ls *lua.LState, resp *ai.ChatResponse) *lua.LTable {
	tbl := ls.NewTable()
	tbl.RawSetString("content", lua.LString(resp.Content))
	tbl.RawSetString("model", lua.LString(resp.Model))
	tbl.RawSetString("finish_reason", lua.LString(resp.FinishReason))

	usage := ls.NewTable()
	usage.RawSetString("prompt_tokens", lua.LNumber(resp.Usage.PromptTokens))
	usage.RawSetString("completion_tokens", lua.LNumber(resp.Usage.CompletionTokens))
	usage.RawSetString("total_tokens", lua.LNumber(resp.Usage.TotalTokens))
	tbl.RawSetString("usage", usage)

	return tbl
}

// pushAIError pushes (nil, err_table) onto the Lua stack and returns 2.
// The err_table has fields: kind, status, message, retry_after.
func pushAIError(ls *lua.LState, e *ai.Error) int {
	ls.Push(lua.LNil)
	ls.Push(aiErrorToTable(ls, e))
	return 2
}

// aiErrorToTable converts a *ai.Error to a Lua table with stable fields.
//
// The "details" field exposes the wrapped underlying error (if any) so
// scripts can surface low-level transport detail (TLS cert issue, DNS
// record, etc.) when the top-level Message isn't enough to diagnose a
// failure. Empty when there is no cause.
func aiErrorToTable(ls *lua.LState, e *ai.Error) *lua.LTable {
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString(e.Kind))
	tbl.RawSetString("status", lua.LNumber(e.Status))
	tbl.RawSetString("message", lua.LString(e.Message))
	tbl.RawSetString("retry_after", lua.LNumber(e.RetryAfter.Seconds()))
	if cause := errors.Unwrap(e); cause != nil {
		tbl.RawSetString("details", lua.LString(cause.Error()))
	} else {
		tbl.RawSetString("details", lua.LString(""))
	}
	return tbl
}
