// Package tracertest holds test helpers for building a [tracer.GenericTracer].
package tracertest

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// Reader is the store surface [tracer.New] reads.
type Reader interface {
	store.EntityReader
	store.RelationReader
}

// Must is [tracer.New] for tests: it panics where New returns an error, so a
// test fixture can build a tracer inline.
func Must(r Reader, world store.WorldScope) *tracer.GenericTracer {
	tr, err := tracer.New(r, world)
	if err != nil {
		panic("tracertest: " + err.Error())
	}
	return tr
}
