package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestQueryStats_AbsentIsNil(t *testing.T) {
	if got := QueryStatsFrom(context.Background()); got != nil {
		t.Fatalf("QueryStatsFrom on a bare context = %v, want nil", got)
	}
}

func TestQueryStats_RecordsCountAndDuration(t *testing.T) {
	ctx, stats := WithQueryStats(context.Background())
	if QueryStatsFrom(ctx) != stats {
		t.Fatal("QueryStatsFrom should return the value WithQueryStats attached")
	}
	stats.Record(3 * time.Millisecond)
	stats.Record(2 * time.Millisecond)
	if got := stats.Queries(); got != 2 {
		t.Errorf("Queries = %d, want 2", got)
	}
	if got := stats.Duration(); got != 5*time.Millisecond {
		t.Errorf("Duration = %v, want 5ms", got)
	}
}

func TestQueryStats_ConcurrentRecordIsExact(t *testing.T) {
	_, stats := WithQueryStats(context.Background())
	const workers, perWorker = 8, 1000
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range perWorker {
				stats.Record(time.Microsecond)
			}
		})
	}
	wg.Wait()
	if got := stats.Queries(); got != workers*perWorker {
		t.Errorf("Queries = %d, want %d", got, workers*perWorker)
	}
	if got := stats.Duration(); got != workers*perWorker*time.Microsecond {
		t.Errorf("Duration = %v, want %v", got, workers*perWorker*time.Microsecond)
	}
}

func TestQueryStats_ChildContextSharesParentStats(t *testing.T) {
	ctx, stats := WithQueryStats(context.Background())
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	QueryStatsFrom(child).Record(time.Millisecond)
	if got := stats.Queries(); got != 1 {
		t.Errorf("a derived context must record into the same stats; Queries = %d", got)
	}
}
