# seqtrace

seqtrace draws numbered sequence diagrams of what rela actually does for a
request. It records every call in rela's own packages, then draws the calls
that cross from one package to another.

It is a diagnostic tool. Normal builds do not include it.

## Quick start

```bash
just seqtrace-build                     # instrumented rela-server in .ignored/seqtrace/
just seqtrace-run /tmp/some-project     # use the app or curl it, then stop it with Ctrl-C
just seqtrace-diagram                   # writes .ignored/seqtrace/diagrams/
open .ignored/seqtrace/diagrams/index.html
```

`index.html` lists the diagrams. Each diagram has its own page with links to
the previous and next one.

Run it against a copy of a project, because writes are real.

Each diagram is also written as a Markdown file with a `mermaid` block. GitHub
and most editors render these.

## Postgres and ACL demo

```bash
just seqtrace-demo
```

This traces a setup that resembles a network deployment and opens the
diagrams in your browser. It runs these steps:

1. Copies the checkout to `.ignored/seqtrace-demo/work/src` and builds there,
   so the checkout itself is never changed. If the checkout has no built
   frontend, the copy gets a stub page instead.
2. Builds an instrumented `rela-server` on the postgres build.
3. Starts a throwaway postgres. It uses a local cluster when `initdb` and
   `pg_ctl` are on `PATH`, and a docker container otherwise. The tenant gets
   its own schema, as in a multi-tenant deployment.
4. Seeds `prototypes/perf/project` with about 1,000 entities. Its `acl.yaml`
   resolves roles through the graph and redacts salaries.
5. Starts the server with identity from a reverse-proxy header
   (`-principal-header X-Forwarded-User`).
6. Sends ten requests as alice (manager), bob (editor) and carol (reader):
   reads, a hidden read, a list, a search, updates, a state transition, a
   denied update, a create and a delete.
7. Stops the server and postgres, and writes one diagram per request to
   `.ignored/seqtrace-demo/diagrams/`.

The scenarios are the `req` lines in `tools/seqtrace/demo/run.sh`. The
environment variables at the top of that script set the ports and the data
size.

The demo uses header identity, not JWT. JWT identity needs the proxy's key
set served over HTTPS, which a local run does not have.

## How it works

1. `seqtrace overlay` copies every Go file in `internal/` and `cmd/` and
   injects calls to the runtime package (`tools/seqtrace`). It writes the
   copies to `.ignored/seqtrace/src/` and lists them in an `overlay.json`.
2. `go build -overlay overlay.json` compiles those copies in place of the
   originals. The working tree does not change. The injected code stays on
   the original lines, so panics and stack traces still show the right line
   numbers.
3. The instrumented binary writes one JSON line per call and per return to
   `SEQTRACE_OUT`.
4. `seqtrace diagram` builds a call tree from that file and writes one Mermaid
   diagram per root call.

## Arguments and results

Call arrows show a short summary of each argument, and return arrows show the
results, for example `GetEntityState(id="TSK-00002")` returning
`*entity.Entity{Type:task ID:TSK-00002}`. A summary identifies a value; it
does not reproduce it:

- Strings are quoted and cut at 48 characters.
- Structs show their type and identifying fields (`Type`, `ID`, `Name` and a
  few others). Slices and maps show their length.
- Errors show their message. A trailing `nil` error is left off.
- A value declared as `any`, such as a property value, shows only its type.
- `context.Context` arguments are left out.
- An argument whose name suggests a secret (`password`, `token`, `key` and
  similar) is shown as `‹redacted›`. So are all arguments and results of a
  function whose name suggests one, such as `resolvePassword`.

Summaries never call a `String` method.

The redaction is a guess based on names, and tracing runs below the ACL. A
trace can hold secrets and values that ACL hides from the requesting user,
from every user the server served while tracing. Handle trace files and
diagrams like a database dump. Trace files are created with mode 0600.

When sibling calls repeat with different values, they still fold into one
loop, and the loop shows `…` in place of the values. Pass `-values=false` to
draw names only.

## Linking work across goroutines

A call normally has as its parent the caller on the same goroutine. When
work moves to another goroutine, the parent is found in one of three ways.
The diagram shows these as async arrows labeled with the method:

| Label       | Case                                                                                                                           |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `[go]`      | A `go` statement in rela code started the goroutine.                                                                           |
| `[closure]` | A function literal runs on a goroutine started by library code, such as `errgroup`, while the call that created it is running. |
| `[ctx]`     | A `context.Context` parameter carries the call ID from another goroutine, for example in a message sent to a worker.           |

Two handoffs do not carry a `ctx` and are not linked yet: `store.Event`
delivery to observers, and jobs in the durable postgres queue. Work started
through them appears as a separate root.

## Options

`just seqtrace-build tags=sqlite` builds a backend variant. It accepts the same
tags as `go build`.

`just seqtrace-run PROJECT ROOT` sets `SEQTRACE_ROOT`, a regexp on
`pkg.Func` such as `internal/dataentry.(*App).handleV1Search`. A call with no
parent starts a trace only if it matches. The default,
`internal/dataentry\.requestStats\.`, matches the outermost HTTP middleware,
so it records requests and skips startup and background loops. Use `.` to
record everything.

`just seqtrace-diagram` accepts these flags:

| Flag        | Meaning                                                                                                                                              |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-root`     | Regexp on `pkg.Func`. Only matching roots are drawn.                                                                                                 |
| `-collapse` | Regexp of packages drawn as part of their caller. The default hides value packages such as `entity` and `metamodel`. Pass `''` to draw all packages. |
| `-depth`    | Maximum nesting of arrows.                                                                                                                           |
| `-min`      | Skip diagrams with fewer arrows than this (default 2).                                                                                               |
| `-names`    | File with one scenario name per recorded request, in order. A line `-` skips that request.                                                           |
| `-values`   | Label arrows with argument and result summaries (default true).                                                                                      |
| `-title`    | Regexp on the function name. The deepest matching call names each diagram (default `\.handle`).                                                      |
| `-in`       | Trace file (default `.ignored/seqtrace/trace.jsonl`).                                                                                                |
| `-out`      | Output directory (default `.ignored/seqtrace/diagrams`).                                                                                             |

## Limits

- Tracing is slow. Every call looks up its goroutine ID. A PATCH on the
  `tickets` project took 3.5 s instead of 0.1 s. The recorded durations show
  where time goes relative to other calls, not real latency.
- Startup is slow too, because calls outside a recorded root still look up
  their goroutine ID. The traced server needs about 45 s to start on the
  `tickets` project. Wait for the "starting server" log line.
- Functions with a `//go:` directive and files that use cgo are not
  instrumented.
- Function literals whose type contains a struct tag are not instrumented.
- Each function replaces its `ctx` parameter with a derived context, so code
  that compares a context with the one it was given sees a different value.
- A `go` statement that starts a named function is matched to the first call
  of that name on a new goroutine within two seconds. Two concurrent spawns of
  the same function can be linked to the wrong parent.
- Calls into code outside rela, such as the standard library, bleve and pgx,
  are not recorded. A callback from that code back into rela is drawn from the
  package that made the outer call.
