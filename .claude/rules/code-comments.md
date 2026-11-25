---
paths:
  - "tools/commentbody/**"
  - "frontend/eslint-rules/**"
  - "frontend/eslint.comments.config.js"
  - "frontend/scripts/comment-body.mjs"
---

# Code comments (TKT-GL18KOA)

- **No comment inside a function body, Go or TypeScript.** When a line needs
  explaining, rename the function or variable, extract a helper, or name the
  constant. Closures count as body. Directives are always fine (`//go:`,
  `//nolint:`, `//plimsoll:`, `eslint-`, `@ts-`, coverage markers). A Go
  group that starts with `#nosec`, `nosemgrep`, `nolint` or `coverage-ignore`
  is allowed whole, so its justification lines may follow it.
- **A function doc is at most 10 prose lines.** Directive lines and blank
  lines do not count. Split the function or trim the doc to its contract.
- **Escape hatch: `why: <reason>`.** For a fact no name can carry, such as a
  library-bug workaround. The reason text is required.
- **Existing findings sit in a baseline that may only shrink:**
  `tools/commentbody/baseline.json` (`just comment-body`) and
  `frontend/comment-body-baseline.json` (`npm run lint:comments`). Counts are
  per file. A higher count fails. A lower one only prints a note, so a cleanup
  cannot break an unrelated PR; run `just comment-body-update` or
  `npm run lint:comments:update` to lock the gain in. Do not raise a count to
  make a build pass.
- CI also runs `go run ./tools/commentbody -against origin/<base>` on PRs. It
  fails when either baseline gains a file or a higher count than the base.
- Out of scope: test files, Go generated files, `frontend/e2e`,
  `frontend/packages`, plain `.js`/`.mjs` files, and TS generated files are not
  excluded by name. A `why:` comment is one group (adjacent `//` lines); the
  first line must start with `why:` and give a reason. Per-line
  `eslint-disable` of this rule does not work, because it is not in the main
  lint config.
