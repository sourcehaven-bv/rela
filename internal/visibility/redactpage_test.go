package visibility

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// rowsSeq yields n rows, then err when it is set. pulled counts the rows
// the consumer drew from it.
func rowsSeq(n int, err error, pulled *int) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for i := range n {
			*pulled++
			if !yield(&entity.Entity{ID: fmt.Sprintf("R-%04d", i), Type: "doc"}, nil) {
				return
			}
		}
		if err != nil {
			yield(nil, err)
		}
	}
}

// pageLog is a pageRedactor that records the size of every page it is given
// and marks each row as redacted.
type pageLog struct{ sizes []int }

func (p *pageLog) redact(_ context.Context, rows []*entity.Entity) []*entity.Entity {
	p.sizes = append(p.sizes, len(rows))
	out := make([]*entity.Entity, len(rows))
	for i, e := range rows {
		c := *e
		c.Properties = map[string]any{"redacted": true}
		out[i] = &c
	}
	return out
}

// Each page costs one prime, so the page count is the query cost of a scan:
// one page up to the first size, then one per headerGateChunk rows.
func TestRedactingSeq_PagesGrowGeometrically(t *testing.T) {
	first := redactPageSizes[0]
	cases := []struct {
		rows int
		want []int
	}{
		{0, nil},
		{1, []int{1}},
		{first, []int{first}},
		{first + 1, []int{first, 1}},
		{first + headerGateChunk + 3, []int{first, headerGateChunk, 3}},
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.rows), func(t *testing.T) {
			var pulled int
			log := &pageLog{}
			var got []string
			for e, err := range redactingSeq(context.Background(), rowsSeq(tc.rows, nil, &pulled), log.redact) {
				if err != nil {
					t.Fatal(err)
				}
				if e.Properties["redacted"] != true {
					t.Fatalf("row %s reached the caller unredacted", e.ID)
				}
				got = append(got, e.ID)
			}
			if len(got) != tc.rows {
				t.Fatalf("yielded %d rows, want %d", len(got), tc.rows)
			}
			if !slices.IsSorted(got) {
				t.Errorf("rows out of order: %v", got)
			}
			if !slices.Equal(log.sizes, tc.want) {
				t.Errorf("pages = %v, want %v", log.sizes, tc.want)
			}
		})
	}
}

// A store error drops the rows buffered before it: the caller gets the
// error, never an unredacted row or a short page that reads as complete.
func TestRedactingSeq_ErrorDropsTheBufferedPage(t *testing.T) {
	boom := errors.New("boom")
	var pulled int
	log := &pageLog{}
	rows, errs := 0, 0
	for e, err := range redactingSeq(context.Background(), rowsSeq(5, boom, &pulled), log.redact) {
		if err != nil {
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v, want boom", err)
			}
			errs++
			continue
		}
		_ = e
		rows++
	}
	if rows != 0 || errs != 1 {
		t.Errorf("got %d rows and %d errors, want 0 and 1", rows, errs)
	}
	if len(log.sizes) != 0 {
		t.Errorf("redacted pages %v after an error", log.sizes)
	}
}

// A consumer that stops early reads ahead at most one page and redacts
// nothing past it.
func TestRedactingSeq_EarlyStopReadsOnePage(t *testing.T) {
	var pulled int
	log := &pageLog{}
	n := 0
	for _, err := range redactingSeq(context.Background(), rowsSeq(10*headerGateChunk, nil, &pulled), log.redact) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		if n == 3 {
			break
		}
	}
	if pulled != redactPageSizes[0] {
		t.Errorf("pulled %d rows from the store for 3, want one page of %d", pulled, redactPageSizes[0])
	}
	if !slices.Equal(log.sizes, []int{redactPageSizes[0]}) {
		t.Errorf("pages = %v, want one", log.sizes)
	}
}

// A nil row from the store is skipped, so neither the prime nor the
// redactor sees one.
func TestRedactingSeq_SkipsNilRows(t *testing.T) {
	src := func(yield func(*entity.Entity, error) bool) {
		for _, e := range []*entity.Entity{nil, {ID: "A", Type: "doc"}, nil} {
			if !yield(e, nil) {
				return
			}
		}
	}
	log := &pageLog{}
	var got []string
	for e, err := range redactingSeq(context.Background(), src, log.redact) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, e.ID)
	}
	if !slices.Equal(got, []string{"A"}) || !slices.Equal(log.sizes, []int{1}) {
		t.Errorf("got %v in pages %v, want [A] in [1]", got, log.sizes)
	}
}

// perRow adapts a single-row redactor to [pageRedactor], with no priming.
func perRow(redact func(context.Context, *entity.Entity) *entity.Entity) pageRedactor {
	return func(ctx context.Context, rows []*entity.Entity) []*entity.Entity {
		out := make([]*entity.Entity, len(rows))
		for i, e := range rows {
			out[i] = redact(ctx, e)
		}
		return out
	}
}

// A store error after a full page keeps the page already yielded and drops
// only the rows buffered since.
func TestRedactingSeq_ErrorAfterAFullPage(t *testing.T) {
	boom := errors.New("boom")
	first := redactPageSizes[0]
	var pulled int
	log := &pageLog{}
	rows := 0
	var gotErr error
	for e, err := range redactingSeq(context.Background(), rowsSeq(first+5, boom, &pulled), log.redact) {
		if err != nil {
			gotErr = err
			continue
		}
		if e.Properties["redacted"] != true {
			t.Fatalf("row %s reached the caller unredacted", e.ID)
		}
		rows++
	}
	if !errors.Is(gotErr, boom) || rows != first {
		t.Errorf("got %d rows and err %v, want %d and boom", rows, gotErr, first)
	}
	if !slices.Equal(log.sizes, []int{first}) {
		t.Errorf("pages = %v, want only the full one", log.sizes)
	}
}
