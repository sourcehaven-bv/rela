package appbuild

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lua"
)

type stubAttachFiler struct{ calls int }

func (s *stubAttachFiler) AttachFile(context.Context, lua.AttachFileRequest) (lua.AttachedFile, error) {
	s.calls++
	return lua.AttachedFile{Name: "x.pdf"}, nil
}

func TestAttachFileHolder(t *testing.T) {
	ctx := context.Background()
	var nilHolder *attachFileHolder
	if _, err := nilHolder.AttachFile(ctx, lua.AttachFileRequest{}); !errors.Is(err, lua.ErrAttachFileUnavailable) {
		t.Errorf("nil holder: err = %v", err)
	}

	s := &Services{attachFile: &attachFileHolder{}}
	if _, err := s.attachFile.AttachFile(ctx, lua.AttachFileRequest{}); !errors.Is(err, lua.ErrAttachFileUnavailable) {
		t.Errorf("empty holder: err = %v", err)
	}
	if err := SetAttachFiler(s, nil); err == nil {
		t.Error("nil filer accepted")
	}
	if err := SetAttachFiler(&Services{}, &stubAttachFiler{}); err == nil {
		t.Error("services without a holder accepted")
	}

	f := &stubAttachFiler{}
	if err := SetAttachFiler(s, f); err != nil {
		t.Fatal(err)
	}
	att, err := s.attachFile.AttachFile(ctx, lua.AttachFileRequest{})
	if err != nil || att.Name != "x.pdf" || f.calls != 1 {
		t.Errorf("installed filer: att %+v err %v calls %d", att, err, f.calls)
	}
}
