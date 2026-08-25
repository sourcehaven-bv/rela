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
resolution: |
  Resolved by PR #1385 (TKT-YH52OM, open against develop), which lands the
  scope-out independently and better than this ticket would have.

  write_file is now capability-gated, moved behind `if r.caps.WriteFile` INSIDE
  registerWriteBindings, and DocumentConfig gains its own `capabilities:` block
  whose godoc states: "Omitting it grants none... A render is a READ surface, so
  the default matters more here than anywhere."

  So after #1385 merges:

  - A document render gets write_file ONLY if the operator declares it. The
    default is already off, which is what this finding asked for.
  - The five graph mutations remain ungated on a document render — untouched by
    #1385, so TKT-PX5YL7's actual subject is unaffected.
  - The categorical distinction this finding drew (filesystem vs graph) is now
    encoded in the code: write_file has its own gate, keyed on a capability,
    while the graph writes are keyed on allowWrites.

  ACTION for TKT-PX5YL7: remove write_file from its scope entirely and depend on
  #1385 rather than re-deciding it. The ticket should name the five graph
  bindings explicitly instead of "the write surface", since the sixth now has a
  different owner and a different mechanism.

  Note #1385 also strengthens this ticket's premise from the other direction: it
  documents that a plain lua.NewReader held http/ai/secrets, so a document
  render was already an exfiltration surface with no writes involved. That is
  independent of the graph-write gap TKT-PX5YL7 covers, but it means "a render
  is a read surface" now has teeth it lacked.
severity: significant
status: addressed
---
