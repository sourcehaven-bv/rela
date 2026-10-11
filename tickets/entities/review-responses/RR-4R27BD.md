---
id: RR-4R27BD
type: review-response
title: Single-target API and naming
finding: A batch API with one caller and a type named like v1.Mention.
severity: nit
resolution: 'Single-target API: MentionSources(target, after, limit) and Resolver.MentionedBy. No new Mention store type; the wire types are v1.MentionedByResponse and v1.MentionSource.'
status: addressed
---
