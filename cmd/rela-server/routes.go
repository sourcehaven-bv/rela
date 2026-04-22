package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Sourcehaven-BV/rela/internal/frontendroutes"
)

// runRoutesCmd implements `rela-server routes [--format table|json]`. It
// prints the Go-side frontend route catalog (the same catalog that
// backs the Lua rela.url helper and the document link rewriter), so
// document authors and operators can see what paths exist.
//
// Exit code: 0 on success, 1 on usage error, 2 on output failure.
func runRoutesCmd(args []string) int {
	fs := flag.NewFlagSet("routes", flag.ContinueOnError)
	format := fs.String("format", "table", "Output format: table or json")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: rela-server routes [--format table|json]")
		fmt.Fprintln(fs.Output(), "")
		fmt.Fprintln(fs.Output(), "Print the frontend route catalog (paths the SPA accepts).")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	routes := frontendroutes.All()

	switch *format {
	case "json":
		if err := writeRoutesJSON(os.Stdout, routes); err != nil {
			fmt.Fprintln(os.Stderr, "rela-server routes:", err)
			return 2
		}
	case "table":
		if err := writeRoutesTable(os.Stdout, routes); err != nil {
			fmt.Fprintln(os.Stderr, "rela-server routes:", err)
			return 2
		}
	default:
		fmt.Fprintf(fs.Output(), "unknown format %q (want table or json)\n", *format)
		fs.Usage()
		return 1
	}
	return 0
}

func writeRoutesJSON(w io.Writer, routes []frontendroutes.Route) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(routes)
}

func writeRoutesTable(w io.Writer, routes []frontendroutes.Route) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tPATH\tPARAMS\tRETURN_TO\tNOTES"); err != nil {
		return err
	}
	for _, r := range routes {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			r.Name, r.Path, luaParamList(r.Params), returnToCell(r.AcceptsReturnTo), r.Notes); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// luaParamList renders the Lua-facing param names for the params column, so
// document authors see the exact keys they should pass to rela.url.
func luaParamList(params []frontendroutes.Param) string {
	if len(params) == 0 {
		return "-"
	}
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Lua
	}
	return strings.Join(names, ", ")
}

func returnToCell(accepts bool) string {
	if accepts {
		return "yes"
	}
	return "-"
}
