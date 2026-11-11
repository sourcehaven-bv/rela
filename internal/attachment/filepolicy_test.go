package attachment

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestPolicyFor(t *testing.T) {
	global := &metamodel.AttachmentsConfig{Allow: []string{"image/png"}, ScanCmd: []string{"g"}}
	for _, tc := range []struct {
		name       string
		meta       *metamodel.Metamodel
		prop       metamodel.PropertyDef
		wantAccept []string
		wantScan   []string
	}{
		{"nil meta", nil, metamodel.PropertyDef{ScanCmd: []string{"p"}}, nil, []string{"p"}},
		{"nil meta, scan off", nil, metamodel.PropertyDef{ScanCmd: []string{"p"}, Scan: metamodel.ScanOff}, nil, nil},
		{"inherits global", &metamodel.Metamodel{Attachments: global}, metamodel.PropertyDef{},
			[]string{"image/png"}, []string{"g"}},
		{"accept replaces global", &metamodel.Metamodel{Attachments: global},
			metamodel.PropertyDef{Accept: []string{"application/pdf"}}, []string{"application/pdf"}, []string{"g"}},
		{"scan off", &metamodel.Metamodel{Attachments: global}, metamodel.PropertyDef{Scan: metamodel.ScanOff},
			[]string{"image/png"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PolicyFor(tc.meta, tc.prop)
			if !equalStrings(got.Accept, tc.wantAccept) || !equalStrings(got.ScanCmd, tc.wantScan) {
				t.Errorf("PolicyFor = %+v; want accept %v, scan %v", got, tc.wantAccept, tc.wantScan)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestChecker_Check(t *testing.T) {
	ctx := context.Background()
	png := realPNG(t, 4, 4)
	pc := func(name string) ProcessContext { return ProcessContext{Property: "shot", FileName: name} }

	t.Run("accepts and sniffs", func(t *testing.T) {
		got, err := NewChecker(nil).Check(ctx, FilePolicy{}, pc("a.png"), png)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Data, png) || got.FileName != "a.png" || got.MIME != "image/png" {
			t.Errorf("Check = %q, %q; want the bytes unchanged as a.png, image/png", got.FileName, got.MIME)
		}
	})
	for _, tc := range []struct {
		name string
		pol  FilePolicy
		file string
		data []byte
	}{
		{"empty", FilePolicy{}, "a.png", nil},
		{"not in accept", FilePolicy{Accept: []string{"application/pdf"}}, "a.png", png},
		{"denied type", FilePolicy{Accept: []string{"text/html"}}, "a.txt", []byte("<html><script>x</script>")},
		{"denied extension", FilePolicy{}, "a.svg", png},
		{"scan without runner", FilePolicy{ScanCmd: []string{"true"}}, "a.png", png},
		{"cmd step without runner", FilePolicy{Transform: []metamodel.TransformStep{{Cmd: []string{"cat"}}}},
			"a.png", png},
		{"transform output outside accept", FilePolicy{
			Accept:    []string{"image/png"},
			Transform: []metamodel.TransformStep{imageStep("jpeg")},
		}, "a.png", png},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewChecker(nil).Check(ctx, tc.pol, pc(tc.file), tc.data); !errors.Is(err, ErrRejected) {
				t.Errorf("Check = %v; want ErrRejected", err)
			}
		})
	}
	t.Run("transform renames and resniffs", func(t *testing.T) {
		pol := FilePolicy{Transform: []metamodel.TransformStep{imageStep("jpeg")}}
		got, err := NewChecker(nil).Check(ctx, pol, pc("a.png"), png)
		if err != nil {
			t.Fatal(err)
		}
		if got.FileName != "a.jpg" || got.MIME != "image/jpeg" {
			t.Errorf("Check = %q, %q; want a.jpg, image/jpeg", got.FileName, got.MIME)
		}
	})
}
