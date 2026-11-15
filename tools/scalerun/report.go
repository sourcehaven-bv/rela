package scalerun

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReportInput is everything the markdown report shows.
type ReportInput struct {
	Config   *Config
	Results  *Results
	Findings []Finding
	// Benchstat is benchstat's text output, shown as is. BenchNote says
	// why the benchmarks did not gate the run; it may come with or without
	// a table.
	Benchstat string
	BenchNote string
	// RunURL links the workflow run; empty locally.
	RunURL string
}

// WriteReport renders the run as markdown: the verdict, the findings, one
// table per scale, the scale ratios and the benchstat comparison.
func WriteReport(w io.Writer, in ReportInput) error {
	var b strings.Builder
	r := in.Results
	b.WriteString("# Postgres scale run\n\n")
	if len(in.Findings) == 0 {
		b.WriteString("**Result: pass.** Every endpoint is within its limits.\n\n")
	} else {
		fmt.Fprintf(&b, "**Result: fail.** %d finding(s).\n\n", len(in.Findings))
		for _, f := range in.Findings {
			fmt.Fprintf(&b, "- **%s**: %s\n", f.Kind, f.Message)
		}
		b.WriteString("\n")
	}
	if r.Commit != "" {
		fmt.Fprintf(&b, "Commit `%s`, ", r.Commit)
	}
	fmt.Fprintf(&b, "started %s, %d measured requests per endpoint and principal.", r.Started.UTC().Format("2006-01-02 15:04 UTC"),
		r.Iterations)
	if in.RunURL != "" {
		fmt.Fprintf(&b, " [Workflow run](%s).", in.RunURL)
	}
	b.WriteString("\n\n")

	for _, s := range r.Scales {
		writeScale(&b, in.Config, s)
	}
	writeRatios(&b, in.Config, r)

	b.WriteString("## Benchmarks\n\n")
	if in.BenchNote != "" {
		b.WriteString(in.BenchNote + "\n")
	}
	if in.Benchstat != "" {
		if in.BenchNote != "" {
			b.WriteString("\n")
		}
		b.WriteString("Compared with the previous passing run by benchstat.\n\n```\n")
		b.WriteString(strings.TrimRight(in.Benchstat, "\n"))
		b.WriteString("\n```\n")
	}
	if in.BenchNote == "" && in.Benchstat == "" {
		b.WriteString("Not run.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeScale(b *strings.Builder, c *Config, s ScaleResult) {
	scale := FormatScale(s.Scale)
	fmt.Fprintf(b, "## Scale %s\n\n", scale)
	fmt.Fprintf(b, "%d entities and %d relations, seeded in %.0f s. Server start %.1f s. RSS %s idle, %s peak.\n\n",
		s.Entities, s.Relations, s.SeedSeconds, s.StartupSeconds, mib(s.RSSIdleKB), mib(s.RSSPeakKB))
	b.WriteString("| Endpoint | As | p50 ms | p95 ms | Limit ms | Max ms | Queries | DB p50 ms | RSS after |\n")
	b.WriteString("|---|---|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, e := range s.Endpoints {
		limit := "-"
		if ep, ok := c.endpoint(e.Endpoint); ok {
			if l, ok := ep.P95MS[scale]; ok {
				limit = fmt.Sprintf("%.0f", l)
			}
		}
		queries := strconv.FormatInt(e.QueriesP50, 10)
		if e.QueriesMax != e.QueriesP50 {
			queries += fmt.Sprintf(" (max %d)", e.QueriesMax)
		}
		name := e.Endpoint
		if e.Error != "" {
			name += " (failed)"
		}
		fmt.Fprintf(b, "| %s | %s | %.1f | %.1f | %s | %.1f | %s | %.1f | %s |\n",
			name, e.Principal, e.P50MS, e.P95MS, limit, e.MaxMS, queries, e.DBP50MS, mib(e.RSSKB))
	}
	b.WriteString("\n")
}

func writeRatios(b *strings.Builder, c *Config, r *Results) {
	base, ok := r.Scale(c.Ratio.BaseScale)
	if !ok {
		return
	}
	for _, s := range r.Scales {
		if s.Scale <= base.Scale {
			continue
		}
		fmt.Fprintf(b, "## Scale %s against scale %s\n\n", FormatScale(s.Scale), FormatScale(base.Scale))
		fmt.Fprintf(b, "p50 ratio; the limit is %.1fx unless the endpoint sets its own, "+
			"and p50 below %.0f ms is not checked. Queries should not grow with the data.\n\n",
			c.Ratio.Max, c.Ratio.FloorMS)
		b.WriteString("| Endpoint | As | p50 ratio | Limit | Queries base -> scaled |\n|---|---|--:|--:|--:|\n")
		for _, e := range s.Endpoints {
			be, ok := base.Find(e.Endpoint, e.Principal)
			if !ok || be.P50MS <= 0 {
				continue
			}
			ep, _ := c.endpoint(e.Endpoint)
			fmt.Fprintf(b, "| %s | %s | %.2fx | %.1fx | %d -> %d |\n", e.Endpoint, e.Principal,
				e.P50MS/be.P50MS, c.ratioMax(ep), be.QueriesMax, e.QueriesMax)
		}
		b.WriteString("\n")
	}
}

const kibPerMiB = 1024

func mib(kb int64) string {
	if kb <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f MiB", float64(kb)/kibPerMiB)
}
