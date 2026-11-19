---
id: RR-KH35M2L
type: review-response
title: A title can show part of a hidden templated display property
finding: changeTitles used buildMention(...).Title, which renders a templated title from the readable placeholders, so a hidden surname gave 'Ada '.
severity: significant
resolution: changeTitles uses WholeDisplayTitle and leaves the id out unless the whole title is readable; audit_titles_test.go pins a templated and a single hidden display property.
status: addressed
---
