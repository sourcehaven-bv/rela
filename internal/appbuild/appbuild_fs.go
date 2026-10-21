//go:build !postgres && !memorybackend && !sqlite

package appbuild

// New builds the services bundle for the default (filesystem) build: an
// fsstore rooted at the project paths plus an in-memory bleve search index.
// The recipe itself is [newFS], which the sqlite build also uses for a
// project that keeps its data in markdown files.
func New(cfg Config, opts ...Option) (*Services, error) {
	return newFS(cfg, opts...)
}
