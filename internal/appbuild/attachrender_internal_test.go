package appbuild

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

type stubAttachRenderer struct{ calls int }

func (s *stubAttachRenderer) AttachRender(context.Context, lua.AttachRenderRequest) (lua.AttachedFile, error) {
	s.calls++
	return lua.AttachedFile{Name: "x.md"}, nil
}

func TestAttachRenderHolder(t *testing.T) {
	ctx := context.Background()
	var nilHolder *attachRenderHolder
	if _, err := nilHolder.AttachRender(ctx, lua.AttachRenderRequest{}); !errors.Is(err, lua.ErrAttachRenderUnavailable) {
		t.Errorf("nil holder: err = %v", err)
	}

	s := &Services{attachRender: &attachRenderHolder{}}
	if _, err := s.attachRender.AttachRender(ctx, lua.AttachRenderRequest{}); !errors.Is(err, lua.ErrAttachRenderUnavailable) {
		t.Errorf("empty holder: err = %v", err)
	}
	if err := SetAttachRenderer(s, nil); err == nil {
		t.Error("nil renderer accepted")
	}
	if err := SetAttachRenderer(&Services{}, &stubAttachRenderer{}); err == nil {
		t.Error("services without a holder accepted")
	}

	r := &stubAttachRenderer{}
	if err := SetAttachRenderer(s, r); err != nil {
		t.Fatal(err)
	}
	att, err := s.attachRender.AttachRender(ctx, lua.AttachRenderRequest{})
	if err != nil || att.Name != "x.md" || r.calls != 1 {
		t.Errorf("installed renderer: att %+v err %v calls %d", att, err, r.calls)
	}
}
