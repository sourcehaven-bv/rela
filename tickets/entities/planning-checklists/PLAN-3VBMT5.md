---
id: PLAN-3VBMT5
type: planning-checklist
title: 'Planning: Lua rela.xml.decode and rela.feed.parse with hardened XML limits'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a hardened XML decoder (`internal/safexml`), a feed parser for
RSS 2.0 / RSS 0.9x, RSS 1.0 (RDF), Atom 1.0 and JSON Feed 1.0/1.1
(`internal/feed`), and two Lua bindings: `rela.xml.decode(string)` and
`rela.feed.parse(string)`. Guide update in docs-project. Out: fetching (scripts
use `http.request`), HTML→markdown of item content (`rela.md.from_html` already
exists), the declarative subscription runner (TKT-RUO1GY), XML encoding, XPath,
namespace-prefix rewriting.

**Acceptance Criteria:**
1. `rela.xml.decode` returns `{name, ns, attrs, children, text}` for a valid
document; invalid XML returns `(nil, err)` with `kind="bad_response"`.
2. Hostile XML is refused with an error, not a crash or blow-up: billion
laughs, external entity (XXE), over-deep nesting, too many elements, too many
attributes, oversize input, unsupported charset.
3. `rela.feed.parse` returns `{format, title, link, description, items}` for
RSS 2.0, RSS 1.0, Atom and JSON Feed; each item has `id, title, link, summary,
content, published, updated, authors, categories`.
4. Dates in RFC 1123/822 variants and RFC 3339 become RFC 3339 UTC strings;
unparseable dates become nil, raw value kept in `published_raw` / `updated_raw`.
5. Item `id` falls back guid → link → nil (never invented).
6. Both bindings are available in every runtime (pure, no I/O), like
`rela.json`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach settled in discussion on TKT-RUO1GY)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `github.com/mmcdole/gofeed`: complete, but brings goquery, goxpp and
json-iterator, and its XML pull parser applies no depth/element caps we can set.
Rejected: we would not control the limits that make it safe.
- `encoding/xml` (stdlib): no DTD processing, no external entities, only
predefined entities unless `Decoder.Entity` is set. Basis of `safexml`.
- `internal/lua/json.go`: convention for pure codec bindings (registration
on the `rela` table, `(nil, err_table)` with `kind="bad_response"`, depth cap
with sentinel). Followed.
- `internal/htmlmd` + `rela.md.from_html`: content conversion for scripts.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `internal/safexml` (leaf, stdlib only): `Decode(data []byte, lim Limits)
(*Node, error)`. `Node{Name, Space string; Attrs []Attr; Children []*Node; Text
string}`. Token loop over `xml.Decoder` with `Strict=true`,
`Entity=xml.HTMLEntity` (static map; feeds use `&nbsp;`), a `CharsetReader` that
accepts only utf-8 / us-ascii / iso-8859-1, and counters for depth, elements and
attributes per element. Directives (`<!DOCTYPE ...>`) are skipped unprocessed
(see design review). Size checked before decoding. `DefaultLimits`: 10 MiB,
depth 64, 100 000 elements, 64 attrs.
- `internal/feed` (depends on safexml): `Parse(data []byte) (Feed,
error)`. JSON when the first non-space byte is `{` (JSON Feed, version URL
checked); otherwise safexml and dispatch on root local name (`rss`, `RDF`,
`feed`). Namespaces matched by URI, not prefix (content:encoded, dc:date,
dc:creator). Value types only; no pointer leaks.
- `internal/lua/xml.go`, `internal/lua/feed.go`: register `rela.xml` and
`rela.feed`; convert Node / Feed to tables.
- `.go-arch-lint.yml`: add both components; `lua` may depend on them;
`feedparse` on `safexml`.

**Files to modify:**
- new: `internal/safexml/{safexml.go,safexml_test.go}`
- new: `internal/feed/{feedparse.go,rss.go,atom.go,jsonfeed.go,dates.go,*_test.go}`
- new: `internal/lua/{xml.go,feed.go,xml_test.go,feed_test.go}`
- `internal/lua/runtime.go` (registration), `.go-arch-lint.yml`
- `docs-project/entities/guides/GUIDE-lua-scripting.md` (+ regenerated `docs/lua-scripting.md`)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The input is an arbitrary string from a script,
typically an untrusted HTTP body. Allowlist: charsets (utf-8, us-ascii,
iso-8859-1), no directives, the five XML entities plus the static HTML entity
map. Hard caps on size, depth, elements and attributes. On violation: error with
a kind and message naming the limit; no partial result.

**Security-Sensitive Operations:** None: no I/O, no file or network access. The
bindings are pure, so registering them in read-only runtimes adds no capability.
Item HTML is returned as a string and never rendered by these packages.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC2: table-driven `safexml` tests: valid doc, namespaces, mixed text;
billion laughs, XXE (`SYSTEM "file:///etc/passwd"`), depth 65, element and
attribute caps, oversize, `encoding="utf-16"` and `windows-1251`.
- AC3–AC5: `feedparse` golden-style tests with fixtures for each format
(RSS 2.0 with content:encoded + dc:creator, RSS 1.0, Atom with html/xhtml
content, JSON Feed 1.1), date variants, id fallbacks.
- AC6 + integration: Lua runtime tests run a script that decodes and parses
fixtures, and a reader runtime confirms the bindings exist there.

**Edge Cases:** Empty string; whitespace only; BOM; CDATA; `&nbsp;`; unknown
root element (error naming it); feed with zero items; item with no title; Atom
`link` with multiple `rel` values (prefer `alternate`); JSON Feed with
`content_html` and `content_text`; numeric JSON Feed id; timezone-less dates.

**Negative Tests:** Malformed XML, unclosed tags, unknown entity, DOCTYPE
present, invalid JSON, JSON that is not a JSON Feed, unsupported root. Each
returns `(nil, err)` in Lua with `kind="bad_response"`; wrong argument type
raises.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Real-world feeds are messy; a strict parser rejects some. Mitigation:
HTML entity map, latin-1 support, lenient dates; strictness stays where it
protects (directives, caps). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] GUIDE-lua-scripting (docs-project) → `docs/lua-scripting.md`: new
`rela.xml` and `rela.feed` sections.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** go-architect review, all applied:
- (significant) Rejecting DOCTYPE bought nothing: encoding/xml never processes a DTD. Directives are now skipped; entity refs to DTD-declared entities fail under Strict.
- (significant) Node lost mixed-content order, so Atom xhtml could not be rebuilt. Switched to ElementTree Text/Tail.
- (significant) Undeclared prefixes leave the prefix in Space. feed accepts the conventional prefix (dc, content, rdf) next to the URI.
- (minor) BOM breaks JSON sniffing: stripped in Decode and Parse.
- (minor) latin-1 labels are windows-1252 in practice: decoded as such; windows-1252 allowed.
- (minor) Size cap also applies to the JSON Feed path.
- (minor) Lua memory: empty attrs/children/text left out.
- (minor) attrs as a map with "{uri}name" keys, xmlns dropped; find/find_all/text helpers added.
- (minor) err.limit names the refused limit.
- (nit) Package renamed feedparse -> feed; private copy of xml.HTMLEntity; control-character test; UTC for zoneless dates documented.
