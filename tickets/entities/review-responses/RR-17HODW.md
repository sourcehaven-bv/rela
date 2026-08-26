---
id: RR-17HODW
type: review-response
title: PendingButton could emit both aria-disabled and native disabled
finding: |-
    `nativelyDisabled` was `props.disabled` and `ariaDisabled` was `props.pending`, computed independently. A caller passing `:disabled="true"` while also pending would get BOTH — silently reintroducing exactly the focus-dropping behaviour the component's own doc comment says aria-disabled exists to avoid, while still asserting the focus-preserving contract in the a11y tree.

    No caller does this today (SettingsView's Upload is the only dual-prop site, and `!stagedLogo` is false during an upload), but nothing prevented it and the component promised otherwise.
severity: minor
resolution: '`ariaDisabled` is now `props.pending && !props.disabled`. Native disabled wins: it has already dropped focus and made the control inert, so adding aria-disabled would only assert a contract the element no longer honours. Covered by a test asserting that a button which is both pending and disabled carries native `disabled` and no `aria-disabled`.'
status: addressed
---
