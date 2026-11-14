// Package scalerun measures rela-server on PostgreSQL at a given data scale
// and decides whether the numbers are acceptable. It backs the weekly
// "Scale run" workflow (.github/workflows/scale-run.yml) and `just scale-run`.
//
// The command in cmd/scalerun owns the processes: it seeds the perf project
// (`rela dev seed`) into a schema of its own, starts rela-server with
// -verbose, and stops it again. This package owns everything that does not
// need a process:
//
//   - [Config] is the checked-in scalerun.yaml: the endpoints to call, the
//     principals to call them as, and the limits.
//   - [Driver] calls each endpoint over HTTP and records latency, the
//     statement count from the Server-Timing header, and server RSS.
//   - [Check] compares [Results] with the limits, the scale ratio and a
//     benchstat comparison, and [WriteReport] renders the markdown report.
//
// Server-Timing carries the statement count only when the server logs at
// Debug (-verbose), because the count is an existence side channel for
// ordinary clients (see requestStats in internal/dataentry). The driver
// therefore treats a missing header as an error rather than as zero.
package scalerun
