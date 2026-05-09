---
id: RR-XNCD3
type: review-response
title: 'Problem framing understates risk: parseDocument silently produces zero-ID entity, not ''explodes'''
finding: 'splitFrontmatter uses bufio.Scanner which tolerates arbitrary binary including NULs. With no `---` delimiter line in ciphertext, frontmatter is empty and body is the binary blob; parseDocument returns a document with empty ID/type. Today''s symptom is silent corruption (zero-ID entity in the index), not ''explodes'' as the plan claims. This means: (a) ticket problem statement should be amended to mention silent-corruption risk; (b) detection MUST happen before any parse-or-index path, including the watcher; (c) audit every call site of readDataFile/ReadFile for entity/relation paths. Reinforces finding about watcher gap.'
severity: significant
resolution: Plan understanding section now correctly states that splitFrontmatter tolerates NULs and an encrypted file silently parses to a zero-ID document. Detection at readDataFile prevents this entirely — parseDocument is never invoked on ciphertext.
status: addressed
---
