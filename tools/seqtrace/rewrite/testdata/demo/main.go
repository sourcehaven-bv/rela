// Command demo exercises every way seqtrace links a call to a parent on
// another goroutine. rewrite_test.go instruments it through an overlay,
// runs it and checks the recorded parents.
package main

import (
	"context"
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
