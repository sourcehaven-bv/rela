package storetest

import (
	"fmt"
	"testing"
)

// Cost is the store work one operation did, as a [Counting] measured it.
type Cost struct {
	// Calls is the number of read calls.
	Calls int
	// Rows is the number of rows those calls returned.
	Rows int
}

// Sub returns c less o. Use it to measure what one step adds to an
// operation, such as a field check on top of a search.
func (c Cost) Sub(o Cost) Cost {
	return Cost{Calls: c.Calls - o.Calls, Rows: c.Rows - o.Rows}
}

// String renders the cost for failure messages.
func (c Cost) String() string {
	return fmt.Sprintf("%d calls, %d rows", c.Calls, c.Rows)
}

// The two store sizes [AssertScaleFree] compares. They are far enough
// apart that a read which scans the store cannot hide in noise. The
// operation's page must be smaller than the small store plus the fixture,
// or a paged read returns fewer rows at the small size.
const (
	ScaleSmall = 100
	ScaleLarge = 1000
)

// AssertScaleFree measures an operation over a store padded with
// [ScaleSmall] rows and over one padded with [ScaleLarge] rows, and fails t
// unless the read calls AND the rows returned are the same at both. It
// returns the large cost so the caller can pin it.
//
// measure builds the fixture with n padding rows, runs the operation once
// and returns its cost plus a detail string ([Counting.String] fits) for the
// failure message. The operation's output must not depend on n.
//
// [Counting] sees the rows the store returns, not the rows a backend scans
// to find them. So a scan shows only when the padding is what the scan
// would return: the same type and face, and edges of the relation types the
// operation reads, pointing at targets only the padding uses. Padding of an
// unrelated type proves nothing.
func AssertScaleFree(t testing.TB, measure func(n int) (Cost, string)) Cost {
	t.Helper()
	small, smallDetail := measure(ScaleSmall)
	large, largeDetail := measure(ScaleLarge)
	AssertSameCost(t, ScaleSmall, small, smallDetail, ScaleLarge, large, largeDetail)
	return large
}

// AssertSameCost fails t unless two measurements of one operation, at
// fixture sizes nSmall and nLarge, have the same calls and the same rows.
// It is [AssertScaleFree] for a test whose fixture sizes are its own.
func AssertSameCost(t testing.TB, nSmall int, small Cost, smallDetail string,
	nLarge int, large Cost, largeDetail string,
) {
	t.Helper()
	if small.Calls != large.Calls {
		t.Errorf("store calls grow with the store: %d at %d, %d at %d (%s | %s)",
			small.Calls, nSmall, large.Calls, nLarge, smallDetail, largeDetail)
	}
	if small.Rows != large.Rows {
		t.Errorf("store rows read grow with the store: %d at %d, %d at %d (%s | %s)",
			small.Rows, nSmall, large.Rows, nLarge, smallDetail, largeDetail)
	}
}
