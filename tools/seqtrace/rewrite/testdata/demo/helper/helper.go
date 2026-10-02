// Package helper stands in for library code (errgroup, net/http) that starts
// goroutines. rewrite_test.go leaves it uninstrumented.
package helper

import "sync"

// Go runs f on a new goroutine and marks wg done when it returns.
func Go(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}
