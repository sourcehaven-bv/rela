---
id: RR-09N4MN
type: review-response
title: Icon compatibility surface is smaller than planned; token contract lives in Go, not tokens.css
finding: |-
    Two plan assumptions are more favourable than documented -- worth correcting so implementation does not build defences that are already there or look in the wrong file.

    1. ICON MAPPING ALREADY EXISTS. The plan calls for building 'a static name -> component allowlist with a documented fallback ... a compatibility surface, not a pure swap.' Sidebar.vue:101 already implements exactly that shape:

        function getIconEmoji(icon?: string): string {
          switch (icon) {
            case 'list': return '\U0001F4CB'
            case 'kanban': return '\U0001F4CA'
            case 'dashboard': return '\U0001F3E0'
            default: return '\U0001F4C4'
          }
        }

    Allowlist switch, three known names, default fallback. The work is changing the return type from string to component and swapping the arms -- not designing a new validation surface. Config icon values are already constrained to this set (there is no Icon field in dataentryconfig at all, so values come from navigation config the SPA already tolerates unknown values for). The security note about 'never dynamic component resolution from a config string' is still correct, but it describes the existing design rather than a new risk.

    Also note Sidebar.vue has hardcoded emoji OUTSIDE the map -- lines 161, 166, 236, 249, 257 (search, analysis, apps, settings, and the theme toggle sun/moon). These are not covered by getIconEmoji and are easy to miss when 'replacing the sidebar emoji'.

    2. THE TYPOGRAPHY CONTRACT IS IN GO. The plan says to guard the '--font-size-* app contract' and lists frontend/src/styles/tokens.css as the file to be careful with. But the typography tokens are not in tokens.css -- they are a Go string constant, appTypographyCSS, at internal/dataentry/apps_css.go:83-95, written into the served CSS at line 126. tokens.css holds only colour. So the contract test belongs on the Go side (apps_css), and edits to tokens.css cannot break the font contract at all.
severity: minor
resolution: Both corrections folded into the plan. (1) The icon work is re-scoped as a return-type change to the existing getIconEmoji allowlist switch (Sidebar.vue:101) rather than a new validation surface; the five hardcoded glyphs OUTSIDE that map — Sidebar.vue lines 161, 166, 236, 249, 257 (search, analysis, apps, settings, theme toggle) — are now called out explicitly in AC 3 so they are not missed. (2) The typography contract is correctly located in the Go constant appTypographyCSS (internal/dataentry/apps_css.go:83-95, emitted at :126), not tokens.css; AC 1 now requires the guarding test on the Go side. tokens.css holds colour only and cannot break the font contract.
status: addressed
---

Verified at `frontend/src/components/common/Sidebar.vue:101-108` and
`internal/dataentry/apps_css.go:74-126`. Both corrections reduce planned work
rather than adding it, but the second one redirects where the guarding test must
live.
