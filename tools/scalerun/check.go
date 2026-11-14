package scalerun

import (
	"fmt"
	"slices"
)

// Finding is one reason a run fails. Kind is "error", "p95", "ratio" or
// "bench".
type Finding struct {
	Kind    string
	Message string
}

// Check compares results with the limits in c and returns every failure.
// bench holds the benchstat comparison; nil means there was no baseline.
//
// The checks:
//   - every request answered the expected status;
//   - every p95 is within the endpoint's limit for that scale (a scale the
//     endpoint names no limit for is not checked);
//   - every scale above the base keeps p50 within the ratio limit of the
//     base scale's p50, unless it is below the floor. The ratio uses p50
//     because p95 over 20 requests is the second-slowest request, and one
//     slow request on a shared runner would double the ratio;
//   - no benchmark got slower by more than the allowed regression.
func Check(c *Config, r *Results, bench []BenchDelta) []Finding {
	var out []Finding
	base, haveBase := r.Scale(c.Ratio.BaseScale)
	for _, s := range r.Scales {
		scale := FormatScale(s.Scale)
		for _, e := range s.Endpoints {
			if e.Error != "" {
				out = append(out, Finding{"error", fmt.Sprintf("scale %s, %s as %s: %s", scale, e.Endpoint, e.Principal, e.Error)})
				continue
			}
			ep, ok := c.endpoint(e.Endpoint)
			if !ok {
				continue
			}
			if limit, ok := ep.P95MS[scale]; ok && e.P95MS > limit {
				out = append(out, Finding{"p95", fmt.Sprintf("scale %s, %s as %s: p95 %.1f ms is above the limit of %.0f ms",
					scale, e.Endpoint, e.Principal, e.P95MS, limit)})
			}
			if !haveBase || s.Scale <= c.Ratio.BaseScale || e.P50MS < c.Ratio.FloorMS {
				continue
			}
			b, found := base.Find(e.Endpoint, e.Principal)
			if !found || b.Error != "" || b.P50MS <= 0 {
				continue
			}
			ratio := e.P50MS / b.P50MS
			if limit := c.ratioMax(ep); ratio > limit {
				out = append(out, Finding{"ratio", fmt.Sprintf(
					"%s as %s: p50 %.1f ms at scale %s is %.1fx the %.1f ms at scale %s (limit %.1fx)",
					e.Endpoint, e.Principal, e.P50MS, scale, ratio, b.P50MS, FormatScale(c.Ratio.BaseScale), limit)})
			}
		}
	}
	for _, d := range bench {
		if d.Significant && d.Delta > c.Bench.MaxRegression {
			out = append(out, Finding{"bench", fmt.Sprintf("%s (%s): %+.1f%% against the previous run (limit +%.0f%%)",
				d.Name, d.Unit, d.Delta*100, c.Bench.MaxRegression*100)})
		}
	}
	slices.SortStableFunc(out, func(a, b Finding) int { return compareKind(a.Kind, b.Kind) })
	return out
}

var kindOrder = []string{"error", "p95", "ratio", "bench"}

func compareKind(a, b string) int {
	return slices.Index(kindOrder, a) - slices.Index(kindOrder, b)
}
