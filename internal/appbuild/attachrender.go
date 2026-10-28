package appbuild

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// attachRenderHolder backs rela.attach_render on the script runtimes appbuild
// builds: schedules and background automations (TKT-1TS55R).
//
// The renderer lives in the data-entry app, which is built after the
// services, and background automation workers may already run by then. So
// the deps carry this holder from the start, and the host installs the
// renderer with [SetAttachRenderer] once the app exists. Until then, and in
// a host with no data-entry app (`rela scheduler`, the CLI), every call
// answers [lua.ErrAttachRenderUnavailable].
//
// Each constructor of [Services] (assembly and NewFromCollaborators) sets
// one; they are separate paths, never both run on one value.
//
// Nil: accepted by the method, which answers like an empty holder. It still
// registers the binding, because a nil pointer in the deps interface is not
// a nil interface, so the constructors never leave it nil.
type attachRenderHolder struct {
	r atomic.Pointer[attachRendererBox]
}

// attachRendererBox lets an interface value sit behind an atomic pointer.
type attachRendererBox struct{ lua.AttachRenderer }

// AttachRender forwards to the installed renderer.
func (h *attachRenderHolder) AttachRender(ctx context.Context, req lua.AttachRenderRequest) (lua.AttachedFile, error) {
	if h == nil {
		return lua.AttachedFile{}, lua.ErrAttachRenderUnavailable
	}
	box := h.r.Load()
	if box == nil {
		return lua.AttachedFile{}, lua.ErrAttachRenderUnavailable
	}
	return box.AttachRender(ctx, req)
}

// SetAttachRenderer installs the renderer behind rela.attach_render for the
// scripts s runs. The data-entry host calls it once its app is built. A
// package function because Services is at its plimsoll cap.
//
// Nil: rejected, because it would make the binding fail at first use rather
// than at wiring.
func SetAttachRenderer(s *Services, r lua.AttachRenderer) error {
	if r == nil {
		return errors.New("appbuild: SetAttachRenderer: renderer is required")
	}
	if s.attachRender == nil {
		return errors.New("appbuild: SetAttachRenderer: services were not fully assembled")
	}
	s.attachRender.r.Store(&attachRendererBox{r})
	return nil
}
