---
id: RR-7JPW40
type: review-response
title: pickRef has no in-flight guard and no rejection handling
finding: 'RlChannelField.vue:70-76. choose() awaits props.pickRef without try/catch, so a rejected picker (dialog torn down, network) becomes an unhandled promise rejection. Repeated clicks open several pickers, and the last one to resolve wins. Fix: keep a local `picking` ref that disables the button (RlButton loading with a pendingLabel), and wrap the call in try/finally. Document that pickRef should resolve null on cancel and never reject.'
severity: minor
resolution: The ref picker now has a picking guard shown as a loading button, a try/catch that treats a rejection as cancel, and a documented resolve-null contract (story PickerInFlight).
status: addressed
---
