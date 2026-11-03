---
id: RR-XPXOZB3
type: review-response
title: fs/mem rename rewrites other bodies below the ACL without per-entity audit
finding: '[security] rewriteMentions edits every live body that has a code span equal to the old id, including rows the renamer cannot read or write. The rename is audited once; each rewritten body gets only a store event. A principal who can create and rename a manual-id entity can pick a common token as the old id and rewrite it across the project.'
severity: significant
resolution: 'Each body the rename rewrites is audited as update-entity triggered by rename:OLD->NEW, so the change is attributable and reviewable. The rewrite itself is the chosen fs/mem design: those backends are VCS backed and resolve old ids by text rewrite instead of a retired-id record.'
status: addressed
---
