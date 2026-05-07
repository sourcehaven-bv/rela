---
id: RR-85N9C
type: review-response
title: Helper href contract is not specified (empty/undefined edge cases)
finding: 'The plan does not define what entityDetailHref returns when the entity type has no list. <a href=''''> reloads the current page; <a href=''#''> jumps to top; :href=''undefined'' omits the attribute and regresses to no-href. Specify the contract: helper always returns a non-empty path. Default fallback: /entity/${type}/${id} (router accepts any type/id pair).'
severity: significant
resolution: 'entityDetailHref contract spelled out: always returns a non-empty path. Floor is /entity/${type}/${id} (router accepts any type/id pair). Helper signature documented in plan.'
status: addressed
---
