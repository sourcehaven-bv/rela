package metamodel

import (
	"strconv"
	"strings"
	"testing"
)

// TestParse_ImageTransformStep_Valid accepts a native image: transform step.
func TestParse_ImageTransformStep_Valid(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
        required: true
      evidence:
        type: file
        transform:
          - image: { reencode: jpeg, quality: 85 }
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("valid image step should parse: %v", err)
	}
	prop := m.Entities["report"].Properties["evidence"]
	if len(prop.Transform) != 1 {
		t.Fatalf("want 1 transform step, got %d", len(prop.Transform))
	}
	step := prop.Transform[0]
	if step.Kind() != "image" {
		t.Errorf("want kind image, got %q", step.Kind())
	}
	if step.Image.ImageReencode() != "jpeg" {
		t.Errorf("want reencode jpeg, got %q", step.Image.ImageReencode())
	}
	if step.Image.Quality != 85 {
		t.Errorf("want quality 85, got %d", step.Image.Quality)
	}
}

// TestParse_ImageTransformStep_DefaultReencode: an image step with no reencode
// defaults to jpeg.
func TestParse_ImageTransformStep_DefaultReencode(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
      avatar:
        type: file
        transform:
          - image: {}
`
	m, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("empty image step should parse: %v", err)
	}
	step := m.Entities["report"].Properties["avatar"].Transform[0]
	if got := step.Image.ImageReencode(); got != "jpeg" {
		t.Errorf("default reencode should be jpeg, got %q", got)
	}
}

// TestParse_ImageTransformStep_UnknownReencode rejects an unsupported output.
func TestParse_ImageTransformStep_UnknownReencode(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
      pic:
        type: file
        transform:
          - image: { reencode: webp }
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "reencode") {
		t.Errorf("error should mention reencode: %v", err)
	}
}

// TestParse_ImageTransformStep_QualityRange rejects an out-of-range quality.
func TestParse_ImageTransformStep_QualityRange(t *testing.T) {
	for _, q := range []int{-3, 0, 850} {
		yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
      pic:
        type: file
        transform:
          - image: { reencode: jpeg, quality: ` + strconv.Itoa(q) + ` }
`
		_, err := Parse([]byte(yaml))
		if q == 0 {
			// 0 means "use default" — valid.
			if err != nil {
				t.Errorf("quality 0 should be valid (default): %v", err)
			}
			continue
		}
		assertError(t, err)
		if !strings.Contains(err.Error(), "quality") {
			t.Errorf("quality %d should be rejected with a quality message: %v", q, err)
		}
	}
}

// TestParse_TransformStep_BothCmdAndImage rejects a step setting both kinds.
func TestParse_TransformStep_BothCmdAndImage(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
      pic:
        type: file
        transform:
          - cmd: [vips, thumbnail]
            image: { reencode: png }
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("error should require exactly one of cmd/image: %v", err)
	}
}

// TestParse_TransformStep_Neither rejects an empty step.
func TestParse_TransformStep_Neither(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      title:
        type: string
      pic:
        type: file
        transform:
          - {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("error should require exactly one of cmd/image: %v", err)
	}
}

// TestParse_ImageTransformStep_OnNonFileRejected: transforms (image or cmd) are
// only valid on file properties.
func TestParse_ImageTransformStep_OnNonFileRejected(t *testing.T) {
	yaml := `version: "1.0"
entities:
  report:
    label: Report
    id_prefix: "REP-"
    properties:
      note:
        type: string
        transform:
          - image: { reencode: jpeg }
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "file") {
		t.Errorf("error should mention file-only: %v", err)
	}
}

// TestTransformStep_Kind covers the Kind discriminator directly.
func TestTransformStep_Kind(t *testing.T) {
	tests := []struct {
		name string
		step TransformStep
		want string
	}{
		{"cmd only", TransformStep{Cmd: []string{"vips"}}, "cmd"},
		{"image only", TransformStep{Image: &ImageStep{}}, "image"},
		{"both", TransformStep{Cmd: []string{"vips"}, Image: &ImageStep{}}, ""},
		{"neither", TransformStep{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.step.Kind(); got != tc.want {
				t.Errorf("Kind() = %q, want %q", got, tc.want)
			}
		})
	}
}
