---
id: RR-IG6RNH
type: review-response
title: mailto parameter stripping was bypassed by a fragment
finding: 'normalizeLinkUrl tested parsed.search, which new URL() only populates when ? precedes #. mailto:a@b.com#frag?bcc=evil@x.com parks the whole parameter string in hash and was stored verbatim, with strippedParams false so the user was not warned. Exploitability is conditional (RFC 6068 puts headers in the query, so a conforming client ignores a post-fragment one), but the control was circumventable.'
severity: minor
resolution: 'Strips both search and hash for mailto, and trims any bare trailing ?/# that assigning '''' leaves in href. Two tests added: the fragment form, and the bare trailing question mark.'
status: addressed
---

**Finding (security review).** The `mailto:` parameter strip tested
`parsed.search` only. `new URL()` populates `search` just when `?` precedes `#`,
so reversing the order hides the parameters in `hash`:

```
normalizeLinkUrl('mailto:a@b.com#frag?bcc=c@d.com&body=hi')
  => { ok: true, url: 'mailto:a@b.com#frag?bcc=c@d.com&body=hi' }
```

`strippedParams` was also `false`, so nothing told the user. The existing test
covered only the `?` form.

Impact is **conditional**: RFC 6068 puts mail headers in the query, so a
strictly-conforming client ignores a post-fragment query. No client was verified
to honour it. But an intended control that can be walked around is worth closing
regardless, and the fix is one line.

**Resolution.** Strip `search` and `hash` both — `mailto:` has no meaningful
fragment. Assigning `''` does not remove a trailing bare `?` or `#` from `href`,
so those delimiters are trimmed rather than trusted; that also tidies
`mailto:a@b.com?`, which previously kept its dangling `?`.

Two tests added: the fragment-hidden parameters, and the bare trailing question
mark.

**Also reviewed and NOT changed:** the same review flagged an unbalanced `)` in
an href truncating the link on save. It does not reproduce under this repo's
pinned `RELA_STRINGIFY_OPTIONS` — measured through both the raw remark pair and
the mounted editor, `https://example.com/path)tok=abc` serializes as
`[L](https://example.com/path\)tok=abc)` and reparses byte-identical. No change
needed.
