---
id: RR-OX9WFS
type: review-response
title: Two icon names described a use site, not the glyph — frozen traps once authored
finding: |-
    icons.ts's own docstring says 'Keys are the public contract — renaming one breaks every project that authored it.' Two of the fifteen then violated that:

    - `analysis` rendered AlertTriangle. An author writing `icon: analysis` on a column labelled 'Analysis' gets a hazard sign.
    - `progress` rendered Wrench, which reads as tooling rather than motion. The prototype used it for 'In Progress' and the column looked like maintenance.

    Both were named for where I happened to use them rather than what they show. Once projects author them neither can be fixed without a migration.

    (`status: CircleDot` and `apps: Blocks` have a milder version of the same smell but are defensible.)
severity: significant
resolution: |-
    Fixed in bdb197f1: `analysis` -> `warning`, `progress` -> `wrench`. Both now describe the glyph, so an author choosing one gets what they expect.

    Done now precisely because it is still free: the only consumer is prototypes/, so this is a rename rather than a migration. After the first real project authors these it would be a breaking change.

    Updated all five sites: the SPA registry, the Go allowlist, the Sidebar's hardcoded Analysis nav item, the prototype YAML, and my own test (which failed on the rename — correctly).
status: addressed
---
