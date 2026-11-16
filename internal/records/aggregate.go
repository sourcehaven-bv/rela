package records

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Bucket is the time bucket an aggregate folds rows into.
type Bucket string

// The buckets. [BucketNone] folds the whole range into one point. Bucket
// starts are in UTC; a week starts on Monday.
const (
	BucketNone  Bucket = ""
	BucketHour  Bucket = "hour"
	BucketDay   Bucket = "day"
	BucketWeek  Bucket = "week"
	BucketMonth Bucket = "month"
)

// ParseBucket parses a bucket name; empty is [BucketNone].
func ParseBucket(s string) (Bucket, error) {
	switch b := Bucket(s); b {
	case BucketNone, BucketHour, BucketDay, BucketWeek, BucketMonth:
		return b, nil
	}
	return "", fmt.Errorf("%w: bucket %q is not hour, day, week or month", ErrInvalid, s)
}

// Start returns the start of the bucket ts falls in.
func (b Bucket) Start(ts time.Time) time.Time {
	ts = ts.UTC()
	switch b {
	case BucketHour:
		return ts.Truncate(time.Hour)
	case BucketDay:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	case BucketWeek:
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
		back := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -back)
	case BucketMonth:
		return time.Date(ts.Year(), ts.Month(), 1, 0, 0, 0, 0, time.UTC)
	case BucketNone:
	}
	return time.Time{}
}

// AggregateQuery folds the rows of one type.
type AggregateQuery struct {
	Type string
	// Owners are the owners whose rows fold. The Service passes only
	// owners the caller may read; a backend trusts the list.
	Owners []string
	// From and To bound TS to [From, To). Zero is unbounded.
	From, To time.Time
	// Field is the decimal or integer field to fold. Empty counts rows
	// only.
	Field string
	// Bucket groups rows by time.
	Bucket Bucket
	// PerOwner keeps owners apart instead of folding them together.
	PerOwner bool
}

// Point is one folded bucket. Owner is set when the query was
// [AggregateQuery.PerOwner]; Start is zero for [BucketNone]. Sum, Min, Max,
// Avg and Latest are exact decimal text, empty when the query names no
// field. Latest is the value of the row with the greatest (TS, ID).
type Point struct {
	Owner  string    `json:"owner,omitempty"`
	Start  time.Time `json:"start,omitzero"`
	Count  int64     `json:"count"`
	Sum    string    `json:"sum,omitempty"`
	Min    string    `json:"min,omitempty"`
	Max    string    `json:"max,omitempty"`
	Avg    string    `json:"avg,omitempty"`
	Latest string    `json:"latest,omitempty"`
}

// fold accumulates one point.
type fold struct {
	owner string
	start time.Time
	count int64
	// n counts the rows that held a number: the average divides by it.
	n        int64
	sum      *big.Rat
	min, max *big.Rat
	latest   *big.Rat
	last     Record
}

// Folder accumulates rows into points. A backend that implements
// [Aggregator] in Go can use it over rows it reads its own way.
type Folder struct {
	q     AggregateQuery
	scale int
	// places is the most decimal places seen, the result scale when scale
	// is negative.
	places int
	points map[string]*fold
}

// NewFolder returns a folder for q. scale is the decimal places of the
// results, negative to use the most places any folded value has; avg gets
// two more.
func NewFolder(q AggregateQuery, scale int) *Folder {
	return &Folder{q: q, scale: scale, points: map[string]*fold{}}
}

// Add folds rec. A row whose field holds no number counts but adds no
// value.
func (f *Folder) Add(rec Record) {
	owner := ""
	if f.q.PerOwner {
		owner = rec.Owner
	}
	start := f.q.Bucket.Start(rec.TS)
	key := owner + "\x00" + start.Format(time.RFC3339)
	p := f.points[key]
	if p == nil {
		p = &fold{owner: owner, start: start}
		f.points[key] = p
	}
	p.count++
	if f.q.Field == "" {
		return
	}
	v, places, err := metamodel.ParseDecimal(rec.Fields[f.q.Field])
	if err != nil {
		return
	}
	f.places = max(f.places, places)
	if p.sum == nil {
		p.sum, p.min, p.max = new(big.Rat), new(big.Rat).Set(v), new(big.Rat).Set(v)
	}
	p.n++
	p.sum.Add(p.sum, v)
	if v.Cmp(p.min) < 0 {
		p.min.Set(v)
	}
	if v.Cmp(p.max) > 0 {
		p.max.Set(v)
	}
	if p.latest == nil || Compare(rec, p.last) > 0 {
		p.latest, p.last = v, rec
	}
}

// Points returns the folded points ordered by owner, then start.
func (f *Folder) Points() []Point {
	scale := f.scale
	if scale < 0 {
		scale = f.places
	}
	out := make([]Point, 0, len(f.points))
	for _, p := range f.points {
		pt := Point{Owner: p.owner, Start: p.start, Count: p.count}
		if p.sum != nil {
			pt.Sum = p.sum.FloatString(scale)
			pt.Min = p.min.FloatString(scale)
			pt.Max = p.max.FloatString(scale)
			pt.Latest = p.latest.FloatString(scale)
			avg := new(big.Rat).Quo(p.sum, new(big.Rat).SetInt64(p.n))
			pt.Avg = avg.FloatString(scale + 2)
		}
		out = append(out, pt)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Owner != out[j].Owner {
			return out[i].Owner < out[j].Owner
		}
		return out[i].Start.Before(out[j].Start)
	})
	return out
}

// MaxAggregateRows bounds the rows [AggregateGeneric] folds in one call. A
// query over more is refused and must narrow its range or owners; a backend
// that folds natively through [Aggregator] has no such bound.
const MaxAggregateRows = 1_000_000

// AggregateGeneric folds the rows of q by paging [Store.List] per owner. It
// is the fallback for a backend without [Aggregator].
func AggregateGeneric(ctx context.Context, s Store, q AggregateQuery, scale int) ([]Point, error) {
	f := NewFolder(q, scale)
	folded := 0
	for _, owner := range q.Owners {
		lq := ListQuery{Type: q.Type, Owner: owner, From: q.From, To: q.To, Limit: MaxPage}
		for {
			page, err := s.List(ctx, lq)
			if err != nil {
				return nil, err
			}
			folded += len(page.Records)
			if folded > MaxAggregateRows {
				return nil, fmt.Errorf("%w: more than %d rows to fold; narrow from, to or owner", ErrInvalid, MaxAggregateRows)
			}
			for _, rec := range page.Records {
				f.Add(rec)
			}
			if page.Next == "" {
				break
			}
			lq.After = page.Next
		}
	}
	return f.Points(), nil
}
