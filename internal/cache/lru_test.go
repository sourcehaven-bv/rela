package cache

import (
	"fmt"
	"sync"
	"testing"
)

func TestLRUGetMissReturnsZero(t *testing.T) {
	c := NewLRU[string, int](4)

	v, ok := c.Get("absent")
	if ok {
		t.Fatalf("expected miss, got hit with value %d", v)
	}
	if v != 0 {
		t.Fatalf("expected zero value 0, got %d", v)
	}
}

func TestLRUPutThenGet(t *testing.T) {
	c := NewLRU[string, int](4)
	c.Put("a", 1)

	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatalf("want a=1, got ok=%v v=%d", ok, v)
	}
}

func TestLRUPutOverwritesValue(t *testing.T) {
	c := NewLRU[string, int](4)
	c.Put("a", 1)
	c.Put("a", 2)

	v, _ := c.Get("a")
	if v != 2 {
		t.Fatalf("want a=2 after overwrite, got %d", v)
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("want len 1, got %d", got)
	}
}

func TestLRUEvictsLeastRecentlyUsed(t *testing.T) {
	c := NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // evicts "a"

	if _, ok := c.Get("a"); ok {
		t.Fatalf("a should have been evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Fatalf("b missing: ok=%v v=%d", ok, v)
	}
	if v, ok := c.Get("c"); !ok || v != 3 {
		t.Fatalf("c missing: ok=%v v=%d", ok, v)
	}
}

func TestLRUGetPromotesEntry(t *testing.T) {
	c := NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // "a" becomes MRU
	c.Put("c", 3) // should evict "b", not "a"

	if _, ok := c.Get("b"); ok {
		t.Fatalf("b should have been evicted (not a)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatalf("a should still be present")
	}
}

func TestLRUPutPromotesEntry(t *testing.T) {
	c := NewLRU[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 11) // update also promotes
	c.Put("c", 3)  // should evict "b", not "a"

	if _, ok := c.Get("b"); ok {
		t.Fatalf("b should have been evicted")
	}
	v, ok := c.Get("a")
	if !ok || v != 11 {
		t.Fatalf("a missing or stale: ok=%v v=%d", ok, v)
	}
}

func TestLRUDelete(t *testing.T) {
	c := NewLRU[string, int](4)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Fatalf("a should have been deleted")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("want len 1, got %d", got)
	}
	c.Delete("missing") // no-op
}

func TestLRUCapacityFloor(t *testing.T) {
	// Capacity <= 0 is treated as 1.
	c := NewLRU[string, int](0)
	c.Put("a", 1)
	c.Put("b", 2)
	if _, ok := c.Get("a"); ok {
		t.Fatalf("a should have been evicted with capacity=1")
	}
}

func TestLRUConcurrentAccess(t *testing.T) {
	c := NewLRU[int, int](64)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := base*100 + j
				c.Put(key, key)
				_, _ = c.Get(key)
			}
		}(i)
	}
	wg.Wait()
	// Size is bounded.
	if got := c.Len(); got > 64 {
		t.Fatalf("len should be bounded by capacity: %d > 64", got)
	}
}

func TestLRUHandlesLargeChurn(t *testing.T) {
	c := NewLRU[string, int](8)
	for i := 0; i < 1000; i++ {
		c.Put(fmt.Sprintf("k%d", i), i)
	}
	if got := c.Len(); got != 8 {
		t.Fatalf("expected capacity-bounded len 8, got %d", got)
	}
}
