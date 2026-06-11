---
id: RR-BZNO
type: review-response
title: Brittle card selector in new vitest cases (text-includes match)
finding: Tests use cards.find((c) => c.text().includes('Duplicates')) to locate cards. If any other card's description ever contains the substring (e.g. an Orphans description mentioning 'duplicated'), .find returns the wrong card and the count assertion fails confusingly. Switch to matching on the .check-title element with startsWith, or look up by index (Duplicates=5, ID Gaps=6 in render order).
severity: significant
resolution: Replaced cards.find((c) => c.text().includes(...)) with a findCard helper that matches on .check-title's leading text via startsWith. Used in both new tests so future card-copy changes can't accidentally match the wrong card.
status: addressed
---
