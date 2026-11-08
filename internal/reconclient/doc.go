// Package reconclient is the client side of rela-to-rela set reconciliation
// (FEAT-CMVZQFI, TKT-0Y566OJ). A [Differ] compares the elements a local
// reader sees with the elements a remote rela serves on
// POST /api/v1/_reconcile, and reports which addresses are only local, only
// remote, or on both sides with a different view hash. It changes nothing.
//
// # Algorithm
//
// The reconciliation is range based, as in Negentropy (RES-G8CYB1R). A span
// is a set of types and an address range [lo, hi). For each open span the
// client compares the count and fingerprint of both sides. A span that
// matches is done. A span that differs is listed when it is small or one side
// is empty, split by type when it names more than one type, and otherwise
// split into buckets at the local side's own element boundaries. The server
// keeps no state; the client drives every split and packs the queries of one
// round into as few requests as the server's limits allow. When nothing
// differs, the run costs one request.
//
// # Views
//
// Each side answers for its own principal. The local side reads through
// [visibility.Elements] under the request bound on ctx, never the raw store,
// so the diff never shows a row or a value the local principal cannot read.
// The remote answers for the principal its credential names. When the two
// principals see different rows or fields, those rows show up as only local,
// only remote, or differing: the diff compares two views, not two databases.
//
// # Trust
//
// Every remote answer is untrusted input. It is validated before use:
// fingerprints with [reconcile.ParseFingerprint], hashes with
// [canonical.ParseDigest], addresses for length, printable characters,
// type, order and range, and pages for a cursor that advances. A violation
// fails the run. The client's own work is bounded by [Options]: round trips,
// listed elements per side, remote address bytes, retries and an overall
// deadline. A remote count only chooses between splitting and listing.
//
// The remote learns some local addresses. A split sends local addresses as
// range bounds, so the remote sees addresses of the local view that it may
// not hold. Two rules limit this. A span with at most 256 local elements is
// listed, never split. The remote counts of the parts of a split must add up
// to its count for the whole, so a remote cannot fake differences to keep the
// client splitting.
//
// A matching fingerprint is not evidence of integrity (see package
// reconcile): a writer in scope can make two different sets match.
//
// # Transport
//
// The [Remote] interface is the transport seam. Package httpremote
// implements it over HTTP; a Remote reports admission refusals as
// [*RefusedError] and a refused range as [*RangeError], so the Differ can
// wait, split or skip.
package reconclient
