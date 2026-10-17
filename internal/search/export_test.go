package search

// SetFieldCheckPageForTest sets how many hits one field check of v judges
// together, so a test can shrink the page and see the per-page cost.
func SetFieldCheckPageForTest(v *Visible, n int) {
	v.page = n
}
