---
id: RR-BH2JHD
type: review-response
title: Docs state a .jar/.xpi rejection guarantee the implementation does not portably provide
finding: The new 'ZIP-container documents' section says 'a ZIP claiming to be a .pdf is still rejected as a polyglot, and so are the executable ZIP containers .jar, .apk and .xpi.' The .pdf half is portable (.pdf is in Go's builtin table). The .jar/.xpi half holds only where the OS MIME database supplies those extensions. An operator will reasonably conclude rela blocks JAR uploads.
severity: nit
resolution: 'The guide now states the guarantee the code actually provides. The two denied groups (executables/installers and macro-enabled office documents) are listed explicitly, and the section says both are matched on the extension itself rather than the implied MIME type, with the reason: Go''s built-in table covers only a handful, so a minimal image resolves most to nothing. The sentence the reviewer flagged is true by construction now rather than conditional on the deploy image.'
status: addressed
---
