//go:build postgres || sqlite

package appbuild

import "github.com/Sourcehaven-BV/rela/internal/metamodel"

// rankingTitles maps each entity type to the property the database search
// backends rank free-text matches by. It is built here because a store must
// not import the metamodel (a store does not depend on an application
// package); what crosses the boundary is plain strings, which the stores bind
// as SQL parameters.
func rankingTitles(meta *metamodel.Metamodel) map[string]string {
	titles := map[string]string{}
	for name := range meta.Entities {
		def := meta.Entities[name]
		if prop := metamodel.RankingTitleProperty(&def); prop != "" {
			titles[name] = prop
		}
	}
	return titles
}
