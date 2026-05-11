---
id: RR-OKQD
type: review-response
title: viewContentBlobs misses entity-card content rendered with refResolver
finding: 'internal/dataentry/api_v1.go viewContentBlobs only collects entry.Content and section.Content, but EntityDetail.vue line 553 also passes refResolver to renderMarkdown(ent.content || '''', refResolver) for each entity inside a content section''s section.entities array (display=''content'' with section.entities?.length, or display=''cards'' if cards ever carried content). buildSections in sections.go line 286 populates SectionEntityData.Content from the underlying entity body for the ''content''/''cards'' display paths, so the SPA does render those bodies as markdown — yet the server never scanned them for mentions. The mismatch is silent: the resolver returns null for every code span in entity-card content, so those code spans stay as <code> while identical content on the entry body becomes a link. This is the central correctness bug in the feature. Fix: extend viewContentBlobs to walk sec.Entities (and sec.Groups[].Entities) appending each ent.Content when HasContent is true. Add a regression test that seeds a view whose section.entities have content with a known-ID code span and asserts mentions contains the ID.'
severity: critical
resolution: Extended viewContentBlobs in internal/dataentry/api_v1.go to walk sec.Entities and sec.Groups[].Entities, appending each ent.Content when ent.HasContent. This is the exact set of blobs EntityDetail.vue passes through renderMarkdown with refResolver.
status: addressed
---
