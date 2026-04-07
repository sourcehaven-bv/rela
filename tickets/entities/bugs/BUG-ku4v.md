---
id: BUG-ku4v
type: bug
title: Markdownlint checks files in nested node_modules
description: 'CI markdownlint was checking e2e/node_modules/**/*.md files causing failures. The ignore pattern #node_modules only excluded root-level node_modules.'
priority: low
effort: xs
why1: Markdownlint failed on files in e2e/node_modules
why2: 'The ignore pattern #node_modules only excludes root-level directory'
why3: The pattern was copied from a simpler project without nested node_modules
why4: Markdownlint config was not revisited when the e2e/ subproject (with its own node_modules) was added
why5: Lint configuration is not treated as part of the change checklist when introducing nested subprojects
prevention: 'Added #**/node_modules to exclude all nested node_modules directories'
status: done
---
