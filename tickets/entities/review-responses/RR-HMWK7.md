---
id: RR-HMWK7
type: review-response
title: Metamodel-default properties in no step were dropped from wizard payloads (divergence from single-page)
finding: initializeDefaults seeds every metamodel property default into formData, including properties in no wizard step. The original wizard prune (keep only activeProperties) dropped such a default from the payload, whereas single-page submits formData as-is. The server only backfills status/template defaults, not arbitrary property defaults, so wizard-created entities could lose defaults single-page ones keep.
severity: significant
resolution: Added wizard.managedProperties (every property named by any step/field, regardless of visibility). pruneWizardHidden now drops a key only if it is managed AND currently inactive; a key no step mentions is left as-is, matching single-page. Unit test pins managedProperties.
status: addressed
---
