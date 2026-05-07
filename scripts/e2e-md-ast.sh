#!/usr/bin/env bash
# End-to-end test of the rela.md inline AST API.
#
# Exercises the structure-preserving AST surface introduced in TKT-9WZIP:
# parse → render fixed point on real markdown content (links, code spans,
# raw HTML, images, autolinks, breaks); the public flatten helper; the
# inline constructors (text, code_span, link_inline, raw_html); and the
# block-level shape (paragraph.inlines, blockquote.children, list-item
# children).
#
# Runs against a built rela binary in a throwaway project. Exits non-zero
# on any mismatch, with the failing case printed.

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
BIN="${REPO}/bin/rela"
DEMO="$(mktemp -d -t rela-md-ast-e2e.XXXXXX)"

cleanup() { rm -rf "${DEMO}"; }
trap cleanup EXIT

say() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31mFAIL\033[0m: %s\n' "$*" >&2; exit 1; }
ok()   { printf '    \033[0;32mOK\033[0m %s\n' "$*"; }

if [[ ! -x "${BIN}" ]]; then
  say "Building rela → ${BIN}"
  (cd "${REPO}" && go build -o "${BIN}" ./cmd/rela)
fi

# Minimal project: just a metamodel; no entities required.
say "Seeding minimal project at ${DEMO}"
cat > "${DEMO}/metamodel.yaml" <<'YAML'
version: "1.0"
namespace: "https://example.com/md-e2e#"
entities:
  note:
    label: Note
    id_prefix: "N-"
    id_type: short
    properties:
      title: {type: string, required: true}
YAML
mkdir -p "${DEMO}/entities" "${DEMO}/relations" "${DEMO}/scripts"

# The Lua script under test. Each assertion uses a mini-helper that
# prints on failure so the shell harness sees the error and exits.
cat > "${DEMO}/scripts/md-ast.lua" <<'LUA'
local fails = 0
local function eq(label, got, want)
    if got ~= want then
        io.stderr:write(string.format("FAIL %s\n  got:  %q\n  want: %q\n",
            label, tostring(got), tostring(want)))
        fails = fails + 1
    end
end
local function truthy(label, cond)
    if not cond then
        io.stderr:write(string.format("FAIL %s (expected truthy)\n", label))
        fails = fails + 1
    end
end

-- 1. parse-shape: paragraph carries `inlines`, not `text`.
do
    local ast = rela.md.parse("hello\n")
    eq("parse-shape.no-text",  ast[1].text,  nil)
    eq("parse-shape.type",     ast[1].type,  "paragraph")
    truthy("parse-shape.inlines", type(ast[1].inlines) == "table")
end

-- 2. round-trip on a kitchen-sink fixture (link, code span, raw HTML,
--    autolink, image, emphasis, strong, strikethrough).
do
    local cases = {
        "see [docs](http://example.com)\n",
        "use `printf`\n",
        "with raw <a name=\"x\">html</a>\n",
        "auto <https://example.com>\n",
        "an image ![alt](pic.png)\n",
        "em *x* strong **y** strike ~~z~~\n",
        "trailing | inside | cells | ok\n",
    }
    for _, src in ipairs(cases) do
        local r1 = rela.md.render(rela.md.parse(src))
        local r2 = rela.md.render(rela.md.parse(r1))
        eq("round-trip "..src, r2, r1)
    end
end

-- 3. flatten() produces legacy text-extraction policy: drops emphasis
--    and link wrappers, keeps `~~` and backticks.
do
    local ast = rela.md.parse("see [docs](url) and `code` and ~~old~~\n")
    local flat = rela.md.flatten(ast[1].inlines)
    eq("flatten.drops-link-wrap", flat:find("docs") ~= nil, true)
    eq("flatten.no-link-syntax",  flat:find("%]%(") == nil, true)
    eq("flatten.preserves-tilde", flat:find("~~old~~") ~= nil, true)
    eq("flatten.preserves-bt",    flat:find("`code`") ~= nil, true)
end

-- 4. inline constructors round-trip through render().
do
    local p = rela.md.paragraph({
        rela.md.text("see "),
        rela.md.link_inline("docs", "/x"),
        rela.md.text(" and "),
        rela.md.code_span("foo()"),
        rela.md.text(" "),
        rela.md.raw_html("<br>"),
    })
    eq("constructors.render", rela.md.render({p}), "see [docs](/x) and `foo()` <br>\n")
end

-- 5. blockquote round-trips with mixed-children content.
do
    local src = "> a paragraph\n>\n> - and a list item\n"
    local r1 = rela.md.render(rela.md.parse(src))
    local r2 = rela.md.render(rela.md.parse(r1))
    eq("blockquote.fixed-point", r2, r1)
end

-- 6. multi-block list items expose `children`.
do
    local ast = rela.md.parse("- first paragraph\n\n  second paragraph\n")
    local item = ast[1].items[1]
    truthy("multi-block.children", type(item) == "table" and type(item.children) == "table")
end

-- 7. headers and first_paragraph use flatten policy.
do
    local ast = rela.md.parse("# A [link](http://x) B\n\nintro [link](http://x) text\n")
    local hs = rela.md.headers(ast)
    eq("headers.flatten", hs[1].title, "A link B")
    eq("first-paragraph.flatten", rela.md.first_paragraph(ast), "intro link text")
end

-- 8. table cell with link survives round-trip.
do
    local src = "| h |\n| --- |\n| see [a](http://x) here |\n"
    local r1 = rela.md.render(rela.md.parse(src))
    local r2 = rela.md.render(rela.md.parse(r1))
    eq("table-cell.fixed-point", r2, r1)
    truthy("table-cell.link-preserved", r1:find("%[a%]") ~= nil)
end

-- 9. C1 regression: pipe inside table cell does not split the row.
do
    local src = "| h |\n| --- |\n| `a|b` |\n"
    local r1 = rela.md.render(rela.md.parse(src))
    local r2 = rela.md.render(rela.md.parse(r1))
    eq("table-cell.pipe-fixed-point", r2, r1)
end

-- 10. C2 regression: code span containing a literal backtick keeps a
--     wide-enough fence on render.
do
    local src = "see `` ` `` inline\n"
    local r1 = rela.md.render(rela.md.parse(src))
    local r2 = rela.md.render(rela.md.parse(r1))
    eq("code-span.backtick-fixed-point", r2, r1)
end

-- 11. resolve_refs: code-span ID is replaced with a link.
do
    local ast = rela.md.parse("see `TKT-1` here\n")
    local out = rela.md.render(rela.md.resolve_refs(ast,
        {["TKT-1"] = "[Fix login](#tkt-1)"}))
    eq("resolve_refs.code-span", out, "see [Fix login](#tkt-1) here\n")
end

-- 12. resolve_refs: bare-prose ID is left alone (only code spans match).
do
    local ast = rela.md.parse("see TKT-1 here\n")
    local out = rela.md.render(rela.md.resolve_refs(ast,
        {["TKT-1"] = "[Fix login](#tkt-1)"}))
    eq("resolve_refs.bare-prose", out, "see TKT-1 here\n")
end

-- 13. resolve_refs: ID inside a fenced code block is NOT replaced.
do
    local src = "```\n`TKT-1`\n```\n"
    local ast = rela.md.parse(src)
    local out = rela.md.render(rela.md.resolve_refs(ast,
        {["TKT-1"] = "[X](#x)"}))
    eq("resolve_refs.code-block-skipped", out, src)
end

if fails > 0 then
    error(string.format("%d assertion(s) failed", fails), 0)
end
print("md-ast e2e: all assertions passed")
LUA

say "Running md-ast.lua via rela script"
cd "${DEMO}"
output="$("${BIN}" script scripts/md-ast.lua 2>&1)" || {
    printf '%s\n' "${output}"
    fail "rela script returned non-zero"
}
printf '%s\n' "${output}"

if ! grep -q "all assertions passed" <<<"${output}"; then
    fail "expected success marker not found"
fi

ok "md-ast e2e: all assertions passed"
