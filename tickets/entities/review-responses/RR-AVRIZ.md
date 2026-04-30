---
id: RR-AVRIZ
type: review-response
title: useConfirmHost cannot detect/refuse a duplicate host mount
finding: 'useConfirmHost returns the module-level state. If a second host mounts (HMR, accidental dual App in tests, future refactor), both bind to the same state and either''s onBeforeUnmount calls settle(false) while the other is still alive. Add hostMounted bookkeeping in useConfirm: throw on second mount, clear on unmount. _resetConfirmForTest must clear it too.'
severity: significant
resolution: 'useConfirm.ts: added module-level hostMounted flag. useConfirmHost throws if a host is already mounted. onBeforeUnmount clears the flag. _resetConfirmForTest clears it too. Tested: second mount throws, remount-after-unmount works.'
status: addressed
---
