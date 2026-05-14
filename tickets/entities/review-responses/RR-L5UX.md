---
id: RR-L5UX
type: review-response
title: Plan says the picker emits {id, type, title} but only id is consumed
finding: 'The select event carries {id, type, title} but the MarkdownEditor parent only needs the `id`. Over-specifying the event contract makes the picker harder to test and harder to reuse. Suggest narrowing the event to `select: [id: string]` so the picker is a generic ''pick an entity ID'' tool. type/title can be added later when a consumer needs them.'
severity: nit
resolution: 'Plan §Approach §1: picker emits select: [id: string] only. Narrow contract; consumers needing type/title re-query.'
status: addressed
---
