package attachment

import (
	"strings"
	"testing"
)

func TestHashBytes(t *testing.T) {
	// SHA-256 of empty data
	got := HashBytes([]byte{})
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("HashBytes([]) = %q, want %q", got, want)
	}

	// SHA-256 of "hello"
	got = HashBytes([]byte("hello"))
	want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("HashBytes(hello) = %q, want %q", got, want)
	}
}

func TestHashReader(t *testing.T) {
	r := strings.NewReader("hello")
	got, err := HashReader(r)
	if err != nil {
		t.Fatalf("HashReader error: %v", err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("HashReader(hello) = %q, want %q", got, want)
	}
}

func TestPathFromHash(t *testing.T) {
	tests := []struct {
		hash string
		ext  string
		want string
	}{
		{"ab3f8c2e9d1a5b6c", ".png", "attachments/ab/ab3f8c2e9d1a5b6c.png"},
		{"cd7e2f1b8a4c3d2e", ".pdf", "attachments/cd/cd7e2f1b8a4c3d2e.pdf"},
		{"1234567890abcdef", ".txt", "attachments/12/1234567890abcdef.txt"},
	}

	for _, tt := range tests {
		got := PathFromHash(tt.hash, tt.ext)
		if got != tt.want {
			t.Errorf("PathFromHash(%q, %q) = %q, want %q", tt.hash, tt.ext, got, tt.want)
		}
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		path     string
		wantHash string
		wantExt  string
		wantOk   bool
	}{
		{"attachments/ab/ab3f8c2e9d1a5b6c.png", "ab3f8c2e9d1a5b6c", ".png", true},
		{"attachments/cd/cd7e2f1b8a4c3d2e.pdf", "cd7e2f1b8a4c3d2e", ".pdf", true},
		{"attachments/12/1234567890abcdef.txt", "1234567890abcdef", ".txt", true},
		// Invalid paths
		{"entities/foo/bar.md", "", "", false},
		{"attachments/ab", "", "", false},
		{"ab3f8c2e.png", "", "", false},
		{"attachments/zz/gg.png", "", "", false}, // gg is not valid hex
	}

	for _, tt := range tests {
		hash, ext, ok := ParsePath(tt.path)
		if ok != tt.wantOk {
			t.Errorf("ParsePath(%q) ok = %v, want %v", tt.path, ok, tt.wantOk)
			continue
		}
		if ok {
			if hash != tt.wantHash {
				t.Errorf("ParsePath(%q) hash = %q, want %q", tt.path, hash, tt.wantHash)
			}
			if ext != tt.wantExt {
				t.Errorf("ParsePath(%q) ext = %q, want %q", tt.path, ext, tt.wantExt)
			}
		}
	}
}

func TestMetadataPath(t *testing.T) {
	got := MetadataPath("attachments/ab/ab3f8c2e.png")
	want := "attachments/ab/ab3f8c2e.png.yaml"
	if got != want {
		t.Errorf("MetadataPath() = %q, want %q", got, want)
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"abcdef", true},
		{"ABCDEF", true},
		{"0123456789", true},
		{"abcdef0123456789", true},
		{"ghijkl", false},
		{"abc def", false},
		{"abc-def", false},
		{"", true}, // empty is technically valid hex
	}

	for _, tt := range tests {
		got := isHex(tt.s)
		if got != tt.want {
			t.Errorf("isHex(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
