---
id: RR-DDZ02R
type: review-response
title: The hand-rolled YAML scanner was unnecessary - yaml is a commonVendor and internal/frontmatter exists
finding: I justified it by an arch-lint rule that does not apply and read the file a third way in a checker whose job is to agree with the store
severity: significant
resolution: Replaced the hand-rolled scanner with frontmatter.Split + yaml.Unmarshal - the same splitter the store uses - plus one line adding frontmatter to analysis.mayDependOn. Its unit test was deleted; the edge cases it covered (CRLF; quoted values; --- in the body) are now end-to-end cases on the check itself where a misparse shows up as the false finding it would cause.
status: addressed
---

My godoc justified the hand-roll by *"arch-lint forbids analysis from depending
on the markdown package"* — true but irrelevant, because I did not need
`markdown`.

`.go-arch-lint.yml` declares `yaml` under `commonVendors`, so every component
may import it today. And `internal/frontmatter` is a declared component whose
package doc describes this exact situation in advance: a dependency-free leaf
returning only strings, existing so `markdown` and `fsstore` can share one
splitter.

Worse than redundant, it was a correctness risk: the check's whole claim is "I
read the file the way rela reads it", and a private scanner made it read the
file a **third** way. Two confirmed divergences — CRLF survived only by accident
of two `TrimSpace` calls, and a UTF-8 BOM made it return zero keys, so a planted
real mismatch passed clean.

Replaced with `frontmatter.Split` + `yaml.Unmarshal`, plus one line adding
`frontmatter` to `analysis.mayDependOn`. The scanner's unit test was deleted and
its edge cases (CRLF, quoted values, `---` in the body) re-added as end-to-end
cases on the check itself, where a misparse shows up as the false finding it
would actually cause.
