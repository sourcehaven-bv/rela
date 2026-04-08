---
id: RR-V6HR
type: review-response
title: Derive Dark from Light has no guard when light is empty — silently wipes Dark column
finding: |-
    frontend/src/views/SettingsView.vue:110 `applyDeriveDark` calls `generateDark(paletteColors.value)` and assigns the result to `paletteDarkColors.value`. If the user's light palette is fully empty (or partially empty), the corresponding dark fields all become empty strings (or invalid values per RR-8PTK). Worse: the Derive button is enabled regardless of whether any light value is set — so a brand-new project where the user clicks Derive immediately erases any dark values they DID type, with the confirm appearing only because there's something to overwrite. The user's intent ('please give me dark colors') is interpreted as 'please erase the dark column'.

    A related UX issue: the inline confirm only fires when `hasAnyDarkValues()` is true. If only ONE dark slot is set, the confirm asks 'overwrite all dark colors?' but in fact the other 7 are empty, so only one slot is actually being replaced. The wording is misleading.

    Fix: (a) disable the Derive button when no light slot is set; (b) on click, validate that ALL light slots that affect dark are valid hex (use the validate-hex regex from `normalizeColorInput`) and show a toast 'Set all light colors before deriving' otherwise; (c) make the confirm say 'overwrite N existing dark colors' with an actual count.

    Bonus paranoia: there's no Undo. If a user derives and immediately realises the result is wrong, they have to either re-import or remember each old value. A simple in-memory undo (one snapshot before derive, one Undo button after) would cost ~20 lines and save support tickets.
severity: significant
resolution: Added `canDeriveDark` computed in SettingsView.vue that checks if any light slot is a valid full hex. Derive button is now `:disabled` when `canDeriveDark` is false, with a tooltip change to 'Set at least one Light color first'. As a belt-and-suspenders, `handleDeriveDark` also calls `uiStore.warning` and bails out if clicked anyway (e.g. via keyboard or a11y path that bypasses :disabled). Combined with RR-8PTK's NaN prevention, an empty-light Derive cannot wipe the dark column with garbage.
status: addressed
---
