---
id: RR-2V3B2U
type: review-response
title: --font-size-md sits inside the frozen contract's 12->14 gap and reads as a contract member
finding: |-
    --font-size-md: 13px (scales.css:63) is SPA-only and undefined in the _rela.css served to custom apps, but its name gives no signal about which side of the boundary it lives on.

    The ramp reads xs, sm, md, base, lg, xl, 2xl, 3xl -- 'md' sorting BEFORE 'base' violates normal naming intuition, so a reader is as likely to conclude 'base' is misnamed as to spot that md is special.

    Concrete failure mode: a dev builds a custom app, copies a var(--font-size-md) usage from the SPA, links _rela.css, and gets the inherited browser default because the variable is undefined in that document. A fallback-less var() on font-size resolves to inherited -- no error, no warning, just wrong-sized text reported months later as 'the app looks different'.

    The mitigation currently is a comment in scales.css and a paragraph in frontend/CLAUDE.md -- neither visible at the call site where the mistake happens.

    Options (reviewer's preference order): (a) rename out of the contract namespace to a ROLE name such as --font-size-dense, which is what the doc comment already calls it ('dense secondary text -- table cells, meta rows, card bodies'); (b) namespace it --spa-font-size-md; (c) at minimum assert in the Go test that appCSSSource does NOT define --font-size-md, pinning the negative side so nobody widens the contract by a drive-by edit.

    The same argument applies with less force to -xs/-2xl/-3xl, but those sit outside the contract's range rather than wedged inside its gap.
severity: significant
resolution: |-
    Fixed in 27bc6ded using the reviewer's preferred option (a): renamed --font-size-md to --font-size-dense. It is a ROLE name rather than a ramp step, which is what the doc comment already described it as ('dense secondary text — table cells, meta rows, card bodies'), so it cannot be mistaken for part of the frozen ramp and no longer sorts confusingly before --font-size-base.

    Also applied option (c) as belt-and-braces: TestAppCSSSource now asserts that appCSSSource does NOT define --font-size-dense, pinning the negative side of the contract so a drive-by edit cannot silently widen it.

    The rationale is recorded at the definition in scales.css and in frontend/CLAUDE.md, including the concrete failure mode (an app author copying the usage gets a silent fallback to the inherited size, with no error).
status: addressed
---
