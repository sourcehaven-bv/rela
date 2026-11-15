---
id: AM-mapped-import-terminal-safe
type: automated-measure
title: Mapped import output filters control characters
description: Puts ESC, CR, a C1 CSI and a bidi override in an import file's key and values and asserts that neither the text plan nor the JSON output contains them.
kind: test
location: internal/cli/import_mapped_test.go (TestImportMapped_ControlCharacters)
status: active
---
