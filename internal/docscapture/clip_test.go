package docscapture

import "testing"

func TestPadAndClamp(t *testing.T) {
	t.Parallel()
	page := rect{W: 1000, H: 2000}
	cases := []struct {
		name string
		r    rect
		pad  float64
		want rect
	}{
		{"interior grows by pad on all sides",
			rect{X: 100, Y: 100, W: 200, H: 100}, 20,
			rect{X: 80, Y: 80, W: 240, H: 140}},
		{"clamps to the left/top edge",
			rect{X: 10, Y: 10, W: 50, H: 50}, 30,
			rect{X: 0, Y: 0, W: 90, H: 90}}, // left/top clamp to 0; right/bottom = 10+50+30
		{"clamps to the right/bottom edge",
			rect{X: 900, Y: 1900, W: 200, H: 200}, 50,
			rect{X: 850, Y: 1850, W: 150, H: 150}}, // right/bottom clamp to page
		{"zero pad is the tight box",
			rect{X: 100, Y: 100, W: 50, H: 50}, 0,
			rect{X: 100, Y: 100, W: 50, H: 50}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := padAndClamp(c.r, c.pad, page)
			if got != c.want {
				t.Errorf("padAndClamp(%+v, %v) = %+v, want %+v", c.r, c.pad, got, c.want)
			}
		})
	}
}
