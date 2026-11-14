package visibility_test

import (
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestElements_StoreCallBudget pins the cost of a range answer for a reader
// with a hidden field: the store calls are the same at 10 and at 50 rows,
// and the same whether or not any hidden value is set. A per-row lookup
// would grow with n; a cost that tracked the hidden values would tell the
// reader which rows hold one.
func TestElements_StoreCallBudget(t *testing.T) {
	types := visibility.ElementTypes{Entities: []string{"ticket"}}
	measure := func(t *testing.T, n int, clearHidden bool) (fp, list, rows int, detail string) {
		t.Helper()
		st := storetest.NewCounting(memstore.New())
		f := newElementsFixture(t, st, n, elementsPolicy())
		if clearHidden {
			for i := range n {
				f.set(fmt.Sprintf("TKT-%03d", i), "salary", nil)
			}
		}
		x := f.elements(nil, nil, 0)
		alice := f.as("alice")

		st.Reset()
		if _, err := x.Fingerprint(alice, types, allEntities); err != nil {
			t.Fatal(err)
		}
		fp, rows = st.Reads(), st.Cost().Rows
		fpDetail := st.String()

		st.Reset()
		if _, err := x.List(alice, types, allEntities, n); err != nil {
			t.Fatal(err)
		}
		return fp, st.Reads(), rows + st.Cost().Rows, "Fingerprint " + fpDetail + "; List " + st.String()
	}

	// The calls are compared across sizes; the rows are not, since a range
	// over every row reads every row by design.
	fp10, list10, _, d10 := measure(t, 10, false)
	fp50, list50, rows50, d50 := measure(t, 50, false)
	if fp10 != fp50 || list10 != list50 {
		t.Errorf("store calls grow with rows:\n  n=10: %s\n  n=50: %s", d10, d50)
	}
	// Whether a hidden value is set must not change the rows read either.
	fpClear, listClear, rowsClear, dClear := measure(t, 50, true)
	if fpClear != fp50 || listClear != list50 || rowsClear != rows50 {
		t.Errorf("store cost depends on whether hidden values are set:\n  set:   %s\n  unset: %s", d50, dClear)
	}
}
