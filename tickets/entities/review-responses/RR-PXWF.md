---
id: RR-PXWF
type: review-response
title: write_file is lumped in with graph mutations, but it is a different thing and probably should stay
finding: 'The ticket treats `rela.write_file` as one of six "write bindings" to remove, then hedges in
  one line that it "deserves separate thought". That hedge is doing too much work — the difference is
  categorical, not a nuance:


  - The five entity/relation bindings mutate the GRAPH through entitymanager: ACL-checked, audited, and
  the thing that makes a render non-idempotent in the way that blocks caching (RR-1DV8RY, TKT-OGR566).

  - `write_file` writes to the FILESYSTEM, confined to `output/` under the project root, path-validated
  with filepath.IsLocal (runtime.go:1265) so no traversal or absolute path escapes. It touches no entity,
  produces no audit row, and is invisible to the store.


  The caching argument does not transfer: a render that drops a side file is still idempotent with respect
  to the graph, and a cache keyed on graph state stays valid. The idempotence argument barely transfers
  either — re-running it overwrites the same path.


  Removing it is therefore a separate decision with a separate rationale, and it is the binding most likely
  to have a real user: a report that emits a CSV alongside its markdown is a plausible thing an operator
  built. Removing the five graph writes breaks nobody in-tree; removing write_file might break someone
  silently.


  The ticket should either (a) scope write_file OUT and say why, or (b) make the case for removing it
  on its own terms rather than by association. As written, an implementer will delete all six because
  they are in the same function.'
severity: significant
status: open
---
