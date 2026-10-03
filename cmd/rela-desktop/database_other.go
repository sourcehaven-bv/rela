//go:build !sqlite

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// hasProjectDatabase reports false: only the sqlite build keeps a project in
// a database.
func hasProjectDatabase(string) bool { return false }

// addDatabaseMenu adds nothing: only the sqlite build has a project database.
func (d *Desktop) addDatabaseMenu(*application.Menu) {}
