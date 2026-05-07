---
id: RR-J7EZX
type: review-response
title: PATCH response body must be merged back into formData; server-side automations set properties beyond what the user typed
finding: |-
    Plan only mutates formData[prop] = value optimistically and never re-syncs from PATCH response. But handleV1UpdateEntity returns the entity *after* automations ran (CLAUDE.md describes set/create_entity automations that fire on property change — e.g., set completed_at when status becomes done). Without merging the response back into formData, automation-derived values are invisible until next page mount.

    Add to plan: after every successful PATCH, merge response.properties back into formData for any property NOT currently dirty per the registry. Same for content. Test: form has automation that sets completed_at when status='done'; after auto-save of status, the rendered completed_at field shows the server-computed date.
severity: significant
resolution: 'useAutoSave.mergeServerResponse runs after every successful PATCH response, merging response.properties and response.content back into formData / content for any property NOT currently dirty per the registry. AC #8 has a Vitest test where a mocked response includes an automation-set property (e.g., completed_at) and asserts it appears in formData after save.'
status: addressed
---
