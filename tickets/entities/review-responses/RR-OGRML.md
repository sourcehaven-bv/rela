---
id: RR-OGRML
type: review-response
title: 'Test coverage gap: triggerEl defaults to null when omitted'
finding: useListActions.ts:92 does scriptErrorStore.show(firstScriptError, triggerEl ?? null). Tests assert positive cases but never assert that calling executeAction without a triggerEl argument calls show(err, null). Easy to regress to undefined silently because fromEl ?? null in the store catches it. Add expect(showSpy.mock.calls[0]?.[1]).toBeNull() to one of the no-trigger tests.
severity: significant
resolution: 'Added expect(showSpy.mock.calls[0]?.[1]).toBeNull() to the ''opens the script-error dialog when one rejection is a ScriptError'' test, where executeAction is called without a triggerEl. Pins down the wire contract: omitted triggerEl becomes explicit null, never undefined.'
status: addressed
---
