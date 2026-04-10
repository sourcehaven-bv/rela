---
id: RR-GVJ06
type: review-response
title: Actions map not included in V1Config API response
finding: V1Config struct (api_v1.go:127-139) does NOT include an Actions field. The frontend never receives action metadata (label, key, confirm, set). Must add Actions map to V1Config and populate in handleV1Config for frontend to render action bars.
severity: significant
resolution: 'Plan updated: add Actions map[string]Action to V1Config struct, populate in handleV1Config from a.Cfg().Actions. Frontend schema store will expose actions for list composables to consume.'
status: addressed
---
