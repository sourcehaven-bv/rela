//go:build !sqlite

package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Sourcehaven-BV/rela/internal/project"
)

// hasProjectDatabase reports false: only the sqlite build keeps a project in
// a database.
func hasProjectDatabase(string) bool { return false }

// addDatabaseMenu adds nothing: only the sqlite build has a project database.
func (m *menuBar) addDatabaseMenu(*application.Menu) {}

// isRelaDocument reports false: a document is a database, which only the
// sqlite build opens.
func isRelaDocument(string) bool { return false }

// documentName is never asked for outside the sqlite build.
func documentName(path string) string { return path }

// documentContext refuses: only the sqlite build opens documents.
func documentContext(string) (*project.Context, error) {
	return nil, errors.New("rela documents need the sqlite build")
}

// storeDocumentConfig refuses: only the sqlite build opens documents.
func storeDocumentConfig(context.Context, string, string, []byte) error {
	return errors.New("rela documents need the sqlite build")
}

// addDocumentMenu adds nothing: only the sqlite build creates documents.
func (m *menuBar) addDocumentMenu(*application.Menu) {}
