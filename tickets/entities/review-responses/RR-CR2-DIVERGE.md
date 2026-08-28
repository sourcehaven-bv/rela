---
id: RR-CR2-DIVERGE
type: review-response
title: customAssetExists and openCustomEntry disagreed on oversize and unreadable files, injecting a link
  that 404s
finding: 'customAssetExists checked existence + not-a-dir; openCustomEntry additionally enforced readable
  + under cap. VERIFIED both divergences end-to-end with a probe: an oversize custom.css gave exists=true
  / openErr=true, as did a mode-0000 file. Result is the exact half-state the feature was designed to
  avoid - the shell injects <link href=/_custom/custom.css> and every page load 404s it with a console
  error and no obvious cause. An operator with a 5MB bundled custom.js hits this, and the docs actively
  encourage import. Worse, TestCustomAssetExists_MatchesOpen CLAIMED to pin this property (''A divergence
  would produce the confusing half-state where the shell links an asset that then 404s'') while only covering
  present/absent/dir/dotfile/symlink - the comment asserted something the test did not establish.'
severity: significant
status: addressed
resolution: customAssetExists now also rejects on info.Size() > maxCustomFileBytes (free from the stat
  it already does) and probes readability with an Open+immediate Close (no read). Added 'oversize file'
  and 'unreadable file' subtests to TestCustomAssetExists_MatchesOpen so the test now establishes what
  its comment always claimed.
---

Raised by `/code-review` of the TKT-IWMETE implementation.
