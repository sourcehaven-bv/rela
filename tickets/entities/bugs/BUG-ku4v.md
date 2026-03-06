---
description: 'CI markdownlint was checking e2e/node_modules/**/*.md files causing failures. The ignore pattern #node_modules only excluded root-level node_modules.'
effort: xs
id: BUG-ku4v
prevention: 'Added #**/node_modules to exclude all nested node_modules directories'
priority: low
status: done
title: Markdownlint checks files in nested node_modules
type: bug
why1: Markdownlint failed on files in e2e/node_modules
why2: 'The ignore pattern #node_modules only excludes root-level directory'
why3: The pattern was copied from a simpler project without nested node_modules
---
