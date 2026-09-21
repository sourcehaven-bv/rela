/**
 * Ranking for the `@` completion menu's results.
 *
 * Its own module, and generic over anything carrying an `id`, because the
 * sandboxed app editor uses the same ranking from a plain IIFE bundle. Reaching
 * into `useMentionMenu` for it would pull Vue and the axios API layer into that
 * bundle, neither of which it can use. Keep every import here Vue-free and
 * transport-free for the same reason.
 *
 * # Rank, do not filter
 *
 * `rankMentions` returns a subset of what it was given, and the only thing that
 * removes a candidate is uFuzzy's own `filter`. An earlier design added a "match
 * quality floor" on top of it (`info.terms >= needed`), which measured out as
 * rejecting *every* prefix match: uFuzzy splits `fancy-` to `["fancy"]`, so a
 * user mid-word scored zero complete terms and the menu blanked on each
 * keystroke until a word was finished. Any floor added later must be pinned by a
 * test over EVERY prefix of a target query, not over complete words — the
 * complete-word fixture is precisely what let that mistake through review.
 *
 * # Never widen the result set
 *
 * Recall is the server's job and it is ACL-gated, so a row invented here would
 * be ungated. The haystack uses the `_title` the server already redacted; no
 * title is ever derived locally.
 */
import uFuzzy from '@leeoniya/ufuzzy'
import { entityDisplayTitle } from '@/utils/entityDisplay'

/**
 * uFuzzy on its DEFAULTS. The needle splits on punctuation, so a hyphenated
 * query like `fancy-rank` arrives as two terms that may match different parts of
 * the haystack.
 *
 * # Why not `intraIns: 1`
 *
 * It looks free — it buys one inserted character inside a term, i.e. tolerance
 * for a single typo (`rankng` → `ranking`). It is not free: it makes matching
 * exponential in needle length whenever a repeated-character run in the needle
 * meets one in the haystack, because the compiled regex backtracks. Measured
 * against this very package, one row, worst-case shape:
 *
 *     needle  24 chars → 27 seconds
 *     needle  32 chars →  3 seconds (shorter haystack)
 *     needle  64 chars → 55 seconds
 *
 * On the defaults every one of those is 0 ms. The work is synchronous on the
 * main thread, and the menu's debounce bounds how OFTEN matching starts, not how
 * long one run takes — so a long title of repeated characters (which no
 * validator rejects, it is just a long title) plus a long query freezes the
 * editor tab while the user has unsaved work. A length bound was considered and
 * rejected: the blowup begins around 16 characters, which is shorter than the
 * queries this feature exists to serve.
 *
 * The motivating case does NOT need it: `fancyreport` matches `FancyReport` on
 * the defaults, because the split is on the needle's punctuation, not on
 * camel-case in the haystack. Only typo tolerance is given up.
 */
const uf = new uFuzzy()

/**
 * Whether uFuzzy's tokenizer saw the whole needle.
 *
 * `uf.split` keeps only what its Latin-oriented `interSplit` matches, and it
 * discards the rest SILENTLY. `日本語-jp` splits to `["jp"]`, and so does
 * `café` → `["caf"]` — this is not only a CJK/Cyrillic problem, it hits ordinary
 * accented European titles too. Ranking on the fragment then drops rows the
 * server matched on the part that was thrown away, which is the "No matches over
 * a good response" failure this guard exists to prevent, reached through a
 * different door.
 *
 * Separators do not count: `fancy-rank` is fully covered even though the hyphen
 * is absent from the terms.
 */
function tokenizerSawWholeNeedle(query: string, terms: string[]): boolean {
  // No terms at all: nothing to rank by, whatever the reason (all-separator
  // `---`, or purely non-Latin). Checked separately because an all-separator
  // query has zero MEANINGFUL characters too, so the coverage comparison below
  // would read as "fully covered" and fall through to a filter that matches
  // nothing.
  if (terms.length === 0) return false
  const covered = terms.join('').length
  const meaningful = query.replace(/[^\p{L}\p{N}]/gu, '').length
  return covered >= meaningful
}

/** Tier 0 exact ID, 1 prefix, 2 everything else. */
function idTier(id: string, upperQuery: string): number {
  const up = id.toUpperCase()
  if (up === upperQuery) return 0
  if (up.startsWith(upperQuery)) return 1
  return 2
}

/**
 * The text each candidate is matched against.
 *
 * Title then ID, and the entity TYPE is deliberately absent: in the SPA the type
 * is a filter chosen from the picker, not a scoring dimension, and including it
 * would let the word "ticket" in a query match every ticket on type alone.
 *
 * `entityDisplayTitle` is also what the menu renders, so the haystack cannot
 * diverge from the visible label. It reads the `_title` the server already
 * redacted.
 */
function haystackFor<T extends { id?: string }>(e: T): string {
  return `${entityDisplayTitle(e as { id: string; _title?: string }) || ''} ${e.id ?? ''}`.trim()
}

/**
 * Orders `items` by match quality against `query`, best first.
 *
 * Fuzzy across the title and the ID together, with an exact or prefix ID match
 * promoted ahead of the fuzzy order — typing a full ID is an unambiguous
 * statement of intent, and uFuzzy alone scores `TKT-ABCD` and `TKT-AB` as peers
 * because it treats the query as the two terms `tkt` and `ab`.
 *
 * Returns a subset of `items`, never a superset and never a new row. Ties keep
 * the server's relevance order.
 *
 * Nil: `items` is returned as-is for a query uFuzzy cannot fully tokenize (see
 * [tokenizerSawWholeNeedle]), since dropping rows there would blank the menu on
 * a perfectly good server response.
 */
export function rankMentions<T extends { id?: string }>(items: T[], query: string): T[] {
  if (!query || items.length === 0) return items

  // uFuzzy could not tokenize the whole needle (non-Latin, punctuation-only,
  // mixed-script, or accented Latin). Pass the server's own ordering through
  // rather than dropping rows it matched on the part uFuzzy discarded.
  if (!tokenizerSawWholeNeedle(query, uf.split(query))) return items

  const haystacks = items.map(haystackFor)
  // `filter` returns null (not an empty array) when nothing matches.
  const idxs = uf.filter(haystacks, query)
  if (idxs === null || idxs.length === 0) return []

  const info = uf.info(idxs, haystacks, query)
  const order = uf.sort(info, haystacks, query)

  const upper = query.toUpperCase()
  return order
    .map((o, rank) => {
      const idx = info.idx[o]
      return { item: items[idx], rank, tier: idTier(items[idx].id ?? '', upper) }
    })
    .sort((a, b) => a.tier - b.tier || a.rank - b.rank)
    .map(({ item }) => item)
}

/**
 * Orders type names by match quality, best first.
 *
 * Separate from [rankMentions] because the haystack is a bare name rather than a
 * composite, and because an unmatched type must drop out: the picker shows a
 * short list of suggestions, so a non-match has no place in it. An empty query
 * keeps the schema's own order so a bare `@` lists every type predictably.
 *
 * SPA-only in practice — the sandboxed app editor has no type picker — but it
 * lives here beside the scorer it shares its guards with.
 */
export function rankTypeNames(names: string[], query: string): string[] {
  if (!query || names.length === 0) return names
  // Same tokenizer-coverage guard as rankMentions; see tokenizerSawWholeNeedle.
  if (!tokenizerSawWholeNeedle(query, uf.split(query))) return names

  const idxs = uf.filter(names, query)
  if (idxs === null || idxs.length === 0) return []

  const info = uf.info(idxs, names, query)
  return uf.sort(info, names, query).map((o) => names[info.idx[o]])
}
