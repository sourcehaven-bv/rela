//go:build memorybackend

package appbuild

// New builds the services bundle for the `memorybackend` build: an
// in-memory memstore plus a zero-dependency LinearSearch index. Data is
// lost on exit; this build exists to prove the backend seam swaps
// cleanly and for quick experiments with no bleve and no database linked.
//
// The recipe is [newInMemory], shared with [NewInMemory]. Unlike
// NewInMemory, this build uses the Config [Discover] built, so its runtime
// state still lands under .rela/ on disk.
func New(cfg Config, opts ...Option) (*Services, error) {
	return newInMemory(cfg, opts)
}
