// Command demo exercises every way seqtrace links a call to a parent on
// another goroutine, and how loop bodies over iterators are attributed.
// rewrite_test.go instruments it through an overlay, runs it and checks the
// recorded parents.
package main

import (
	"context"
	"iter"
	"sync"

	"github.com/Sourcehaven-BV/rela/tools/seqtrace"
	"github.com/Sourcehaven-BV/rela/tools/seqtrace/rewrite/testdata/demo/helper"
)

type job struct {
	ctx  context.Context
	name string
}

func main() {
	defer seqtrace.Flush()
	run(context.Background())
}

func run(ctx context.Context) {
	iterate()

	var wg sync.WaitGroup

	// go statement with a named function: Spawn, claimed by name.
	wg.Add(1)
	go spawned(&wg)

	// go statement with a literal: the literal adopts its creator.
	wg.Add(1)
	go func() {
		defer wg.Done()
		leaf()
	}()

	// A literal started by code that is not instrumented (here: a helper
	// that stands in for errgroup) adopts its still-running creator.
	helper.Go(&wg, func() { leaf() })

	// A message carrying ctx to a long-lived worker: the handler's parent
	// comes from ctx, not from the worker loop below it on the stack.
	jobs := make(chan job)
	done := make(chan struct{})
	go worker(jobs, done)
	jobs <- job{ctx: ctx, name: "a"}
	close(jobs)
	<-done
	wg.Wait()
}

// iterate ranges over iterators. Calls in a loop body belong to the
// function with the loop, even though the iterator's yield runs them.
func iterate() {
	for range nums() {
		bodySimple()
	}
	for range viaHelper() {
		bodyHelper()
	}
	for range adapt(nums()) {
		bodyAdapter()
	}
	for range nums() {
		break
	}
	afterBreak()
}

// nums yields 1, 2 and 3, then calls producerTail unless stopped early.
func nums() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; i <= 3; i++ {
			if !yield(i) {
				producerTail()
				return
			}
		}
	}
}

// viaHelper hands yield to a helper that calls it.
func viaHelper() iter.Seq[int] {
	return func(yield func(int) bool) { emit(yield) }
}

func emit(yield func(int) bool) { yield(1) }

// adapt yields from inside its own range loop over seq.
func adapt(seq iter.Seq[int]) iter.Seq[int] {
	return func(yield func(int) bool) {
		for v := range seq {
			adapterBody()
			if !yield(v) {
				return
			}
		}
	}
}

func bodySimple()   {}
func bodyHelper()   {}
func bodyAdapter()  {}
func adapterBody()  {}
func afterBreak()   {}
func producerTail() {}

func spawned(wg *sync.WaitGroup) {
	defer wg.Done()
	leaf()
}

func worker(jobs <-chan job, done chan<- struct{}) {
	for j := range jobs {
		handle(j.ctx, j.name)
	}
	close(done)
}

func handle(ctx context.Context, name string) {
	_ = ctx
	_ = name
	leaf()
}

func leaf() {}
