package fsstore

import "testing"

func TestIsGitCryptEncrypted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "exact 10-byte magic header",
			data: []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00},
			want: true,
		},
		{
			name: "magic header followed by ciphertext",
			data: append([]byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}, []byte("ciphertext bytes")...),
			want: true,
		},
		{
			name: "magic header followed by conflict-marker bytes",
			// Regression: ordering must put magic-header check BEFORE
			// the conflict-marker scan in parseDocument.
			data: append([]byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T', 0x00}, []byte("xxx<<<<<<< xxx")...),
			want: true,
		},
		{
			name: "shorter than header",
			data: []byte{0x00, 'G', 'I', 'T', 'C', 'R', 'Y', 'P', 'T'},
			want: false,
		},
		{
			name: "normal markdown frontmatter",
			data: []byte("---\nid: FEAT-001\ntype: feature\n---\n\n# Title\n"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isGitCryptEncrypted(tt.data); got != tt.want {
				t.Errorf("isGitCryptEncrypted(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}
