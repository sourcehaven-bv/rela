package channels

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func newFile(prop, name string, size int) NewFile {
	return NewFile{Property: prop, Name: name, MIME: "text/plain", Data: bytes.Repeat([]byte("x"), size)}
}

func TestPrepareFiles(t *testing.T) {
	got, err := PrepareFiles(nil)
	if err != nil || got != nil {
		t.Fatalf("PrepareFiles(nil) = %v, %v", got, err)
	}
	in := []NewFile{newFile("shot", "a.txt", 3), newFile("shot", "b.txt", 5), newFile("doc", "a.txt", 1)}
	got, err = PrepareFiles(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(in) {
		t.Fatalf("got %d files, want %d", len(got), len(in))
	}
	seen := map[string]bool{}
	for i, f := range got {
		if !ValidFileID(f.ID) || seen[f.ID] {
			t.Errorf("file %d: id %q is invalid or repeated", i, f.ID)
		}
		seen[f.ID] = true
		if f.Property != in[i].Property || f.Name != in[i].Name || f.MIME != in[i].MIME || f.Size != int64(len(in[i].Data)) {
			t.Errorf("file %d = %+v, want the fields of %s/%s", i, f, in[i].Property, in[i].Name)
		}
	}
}

func TestPrepareFiles_Refused(t *testing.T) {
	half := MaxMessageFileBytes/2 + 1
	for _, tc := range []struct {
		name  string
		files []NewFile
		want  error
	}{
		{"too many", []NewFile{newFile("p", "1", 1), newFile("p", "2", 1), newFile("p", "3", 1),
			newFile("p", "4", 1), newFile("p", "5", 1)}, ErrTooLarge},
		{"over the message cap", []NewFile{newFile("p", "1", half), newFile("p", "2", half)}, ErrTooLarge},
		{"over the file cap", []NewFile{newFile("p", "1", MaxFileBytes+1)}, ErrTooLarge},
		{"same name twice", []NewFile{newFile("p", "a", 1), newFile("p", "a", 1)}, ErrInvalidMessage},
		{"bad property", []NewFile{newFile("Bad-Prop", "a", 1)}, ErrInvalidMessage},
		{"empty", []NewFile{newFile("p", "a", 0)}, ErrInvalidMessage},
		{"no MIME", []NewFile{{Property: "p", Name: "a", Data: []byte("x")}}, ErrInvalidMessage},
		{"MIME with a control", []NewFile{{Property: "p", Name: "a", MIME: "text/\n", Data: []byte("x")}},
			ErrInvalidMessage},
		{"empty name", []NewFile{newFile("p", "", 1)}, ErrInvalidMessage},
		{"long name", []NewFile{newFile("p", strings.Repeat("a", MaxFileNameBytes+1), 1)}, ErrInvalidMessage},
		{"invalid UTF-8", []NewFile{newFile("p", "a\xff", 1)}, ErrInvalidMessage},
		{"dot dot", []NewFile{newFile("p", "..", 1)}, ErrInvalidMessage},
		{"slash", []NewFile{newFile("p", "a/b", 1)}, ErrInvalidMessage},
		{"backslash", []NewFile{newFile("p", `a\b`, 1)}, ErrInvalidMessage},
		{"control", []NewFile{newFile("p", "a\tb", 1)}, ErrInvalidMessage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PrepareFiles(tc.files); !errors.Is(err, tc.want) {
				t.Fatalf("PrepareFiles = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFileOf(t *testing.T) {
	files, err := PrepareFiles([]NewFile{newFile("p", "a", 1)})
	if err != nil {
		t.Fatal(err)
	}
	m := Message{Seq: 1, Files: files}
	if f, ok := FileOf(m, files[0].ID); !ok || f != files[0] {
		t.Fatalf("FileOf = %+v, %v", f, ok)
	}
	if _, ok := FileOf(m, strings.Repeat("a", fileIDLen)); ok {
		t.Fatal("an unlisted id was found")
	}
	m.Removed = &Removal{}
	if _, ok := FileOf(m, files[0].ID); ok {
		t.Fatal("a removed message lists a file")
	}
}

func TestValidFileID(t *testing.T) {
	for id, want := range map[string]bool{
		strings.Repeat("a", fileIDLen):   true,
		strings.Repeat("a", fileIDLen-1): false,
		strings.Repeat("A", fileIDLen):   false,
		"../" + strings.Repeat("a", 23):  false,
	} {
		if got := ValidFileID(id); got != want {
			t.Errorf("ValidFileID(%q) = %v, want %v", id, got, want)
		}
	}
}
