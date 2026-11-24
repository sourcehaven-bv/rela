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
  `//nolint:`, `//plimsoll:`, `eslint-`, `@ts-`, coverage markers).
- **A function doc is at most 10 prose lines.** Directive lines and blank
  lines do not count. Split the function or trim the doc to its contract.
- **Escape hatch: `why: <reason>`.** For a fact no name can carry, such as a
  library-bug workaround. The reason text is required.
- **Existing findings sit in a baseline that may only shrink:**
  `tools/commentbody/baseline.json` (`just comment-body`) and
  `frontend/comment-body-baseline.json` (`npm run lint:comments`). Counts are
  per file. A higher count fails. A lower one fails too, until you run
  `just comment-body-update` or `npm run lint:comments:update`, so a cleanup
  cannot be undone later. Do not raise a count to make a build pass.
- Test files, generated files and `frontend/e2e` are out of scope.
