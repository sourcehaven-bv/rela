package appbuild

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

// attachFileHolder backs rela.attach_file on the script runtimes appbuild
// builds: schedules, flow steps and background automations (TKT-36DHKH8).
// It is late-bound for the reason [attachRenderHolder] is, and the host
// installs the filer with [SetAttachFiler]. Until then, and in a host with
// no data-entry app, every call answers [lua.ErrAttachFileUnavailable].
//
// Nil: accepted by the method, which answers like an empty holder; the
// constructors never leave it nil, as for [attachRenderHolder].
type attachFileHolder struct {
	f atomic.Pointer[attachFilerBox]
}

// attachFilerBox lets an interface value sit behind an atomic pointer.
type attachFilerBox struct{ lua.AttachFiler }

// AttachFile forwards to the installed filer.
func (h *attachFileHolder) AttachFile(ctx context.Context, req lua.AttachFileRequest) (lua.AttachedFile, error) {
	if h == nil {
		return lua.AttachedFile{}, lua.ErrAttachFileUnavailable
	}
	box := h.f.Load()
	if box == nil {
		return lua.AttachedFile{}, lua.ErrAttachFileUnavailable
	}
	return box.AttachFile(ctx, req)
}

// SetAttachFiler installs the filer behind rela.attach_file for the scripts
// s runs. The data-entry host calls it once its app is built. A package
// function because Services is at its plimsoll cap.
//
// Nil: rejected, because it would make the binding fail at first use rather
// than at wiring.
func SetAttachFiler(s *Services, f lua.AttachFiler) error {
	if f == nil {
		return errors.New("appbuild: SetAttachFiler: filer is required")
	}
	if s.attachFile == nil {
		return errors.New("appbuild: SetAttachFiler: services were not fully assembled")
	}
	s.attachFile.f.Store(&attachFilerBox{f})
	return nil
}
