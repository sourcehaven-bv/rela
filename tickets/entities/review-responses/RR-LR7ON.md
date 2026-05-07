---
id: RR-LR7ON
type: review-response
title: '`relations: {tagged: {}}` (data field absent) silently deletes all tagged edges'
finding: |-
    JSON-decoding `{"tagged":{}}` gives `update.Data = nil`. Handler treats nil and [] identically — drops every existing edge. JSON:API §9 says nothing about `{}` (no `data` key); only `data: []` and `data: null` are defined. A frontend auto-saver that builds the body via object spread before fetch completes hits this and nukes user data.

    This is RR-6YF8F's `data: []` footgun manifesting through a different shape that the user discipline note doesn't cover.

    Fix: change V1RelationsUpdate.Data to *[]V1ResourceIdentifier (pointer); reject `data` field absent as 400 shape error.

    ```go
    type V1RelationsUpdate struct {
        Data *[]V1ResourceIdentifier `json:"data"`
    }
    // in handler:
    if update.Data == nil {
        return shape error: /relations/<type>/data: required
    }
    desired := *update.Data
    ```

    Add test for `{"tagged":{}}` returning 400.
severity: critical
resolution: 'V1RelationsUpdate now has a custom UnmarshalJSON that distinguishes three cases: data field absent (DataPresent=false → 400), data: null or [] (DataPresent=true, Data=[] → remove all), data: [...] (upsert). Test TestV1Patch_DataFieldAbsentIs400 confirms `{"tagged": {}}` returns 400. Test TestV1Patch_DataNullEquivalentToEmpty confirms data: null still treats as empty.'
status: addressed
---
