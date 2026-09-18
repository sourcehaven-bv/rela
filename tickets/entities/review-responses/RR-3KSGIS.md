---
id: RR-3KSGIS
type: review-response
title: Acceptance criterion 2 was argued from the import graph, not tested
finding: '''The two editors cannot serialize a body differently. Test: the modules are IMPORTED, not copied.''
  An import graph is not a serialization guarantee: each editor still builds its own Milkdown instance,
  and the plugin set decides the output. Both files separately computed commonmark.filter(...) to drop
  remarkPreserveEmptyLinePlugin, and both separately merged RELA_STRINGIFY_OPTIONS, with a comment in
  each asserting the two must match. Asserted-in-a-comment across two files is exactly the markdownContentMirror.test.ts
  situation this ticket deleted.'
severity: minor
resolution: 'Extracted editorPreset.ts holding RELA_COMMONMARK and configureRelaSerializer; both editors
  now import them and neither computes either locally. Added editorPreset.test.ts: it pins the preset''s
  output over block constructs, pins that exactly remarkPreserveEmptyLinePlugin and nothing else is dropped,
  and greps both editor sources to fail if either rebuilds the markdown stack itself.'
status: addressed
---
