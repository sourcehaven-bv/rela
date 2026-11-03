---
id: RR-8PVIR2
type: review-response
title: Spurious cursor when the last page is exactly full
finding: Next was set whenever the page filled, so an entity with exactly limit sources reported more.
severity: nit
resolution: 'The resolver looks for one served item past the page before setting Next. Test: TestResolver_MentionedByExactPage.'
status: addressed
---
