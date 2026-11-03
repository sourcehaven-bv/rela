---
id: RR-4GRVSU
type: review-response
title: Optional boolean becomes explicit false on edit; a required boolean cannot be false without two toggles
finding: channelValues.ts:163 fromPropValue returns false for an absent boolean, so an edit sends `false` and turns 'absent' into 'false' (visible to `when:` and to triggers that test presence). toPropValue:85-86 refuses a required boolean that is undefined, so unless the composer seeds false, a user who wants 'no' must tick and untick. Make absent round-trip as absent (return undefined, and let the checkbox render undefined as unchecked). For required booleans, have the composer seed false explicitly and document it in the module header.
severity: minor
resolution: An absent optional boolean stays absent on edit, initialValues seeds a required boolean with false, and the module header documents both rules.
status: addressed
---
