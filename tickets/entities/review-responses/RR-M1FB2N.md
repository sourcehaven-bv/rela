---
id: RR-M1FB2N
type: review-response
title: 'Reduced-motion fallback: match the repo convention and avoid static dots'
finding: 'Components here disable motion under @media (prefers-reduced-motion: reduce) (for example rela-components/src/components/board/RlBoardCard.vue:198). Opting in under no-preference is also correct, but the plan should pick one convention deliberately. Three static dots read as truncated text, so the reduced-motion story should show a static icon or a dot-free tag.'
severity: nit
resolution: 'The indicator follows the repo convention: animated by default, and under prefers-reduced-motion: reduce the dots are hidden and a static icon shows; the reduced-motion story shows the icon.'
status: addressed
---
