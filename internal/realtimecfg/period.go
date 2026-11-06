package realtimecfg

import "github.com/Sourcehaven-BV/rela/internal/flow"

// Period is a span of time written as a count and a unit. The engine owns
// it: see [flow.Period].
type Period = flow.Period

// ParsePeriod reads a period such as "15m", "30d" or "7y" (see
// [flow.ParsePeriod]).
func ParsePeriod(s string) (Period, error) { return flow.ParsePeriod(s) }
