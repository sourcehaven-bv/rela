---
id: BUG-EXFOEI3
type: bug
title: Indented --- line closes frontmatter early and truncates properties
description: 'frontmatter.Split treated any line that trims to --- as the closing fence. yaml.v3 writes a block scalar containing a --- line (a value or a key with a --- line) with that line indented, so the property was truncated or renamed and the rest of the YAML leaked into the body. Issue #2160.'
priority: medium
effort: s
why1: 'fsstore read back the key ---\n as an empty key: the frontmatter block ended at the indented --- line inside the key block scalar.'
why2: frontmatter.Split compared strings.TrimSpace(line) to the delimiter, so an indented --- matched.
why3: The splitter tolerated stray whitespace on both fences for hand-authored files, without considering that YAML content lines are indented and may themselves be ---.
why4: No round-trip test covered values or keys whose text contains the fence line; earlier fuzz fixes targeted the YAML emitter rather than the fence splitter.
why5: 'Framing and payload were not separated by a rule the payload cannot break: column 0 is the position YAML block-scalar content cannot occupy, and the splitter did not use it.'
prevention: The closing fence must start at column 0. The fuzz input is kept as a seed and TestSplit pins an indented --- as content. Delimiter-based framing must define a fence the framed payload cannot emit.
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

An indented --- line inside a YAML block scalar closed the frontmatter block and
silently truncated or renamed a property. Found by FuzzPropertyValuesTypeZoo
(issue #2160).
