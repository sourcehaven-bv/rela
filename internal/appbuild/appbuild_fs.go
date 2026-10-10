//go:build !postgres && !memorybackend && !sqlite

package appbuild

import "github.com/Sourcehaven-BV/rela/internal/app"

// New builds the services bundle for the default (filesystem) build: an
// fsstore rooted at the project paths plus an in-memory bleve search index.
// The recipe itself is [newFS], which the sqlite build also uses for a
// project that keeps its data in markdown files.
func New(cfg Config, opts ...Option) (*Services, error) {
	return newFS(cfg, opts...)
}

// DropStoreIndex removes the persisted store index of a project that a
// replaced generation has just closed over; see [app.FSFactory.DropStoreIndex].
func DropStoreIndex(svc *Services) error {
	return (&app.FSFactory{FS: svc.FS(), Paths: svc.Paths()}).DropStoreIndex()
}
