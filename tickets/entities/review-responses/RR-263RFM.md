---
id: RR-263RFM
type: review-response
title: Empty non-nil TypeScope.Faces failed open on pg and the generic Query path
finding: The doc said an empty list admits none; AdmitsFace agreed, but FaceIn and the pg builder read an empty list as every face.
severity: significant
resolution: ValidateScope now refuses an empty non-nil face allowlist with ErrScope, on every backend. RunFacedVisibleSearchTests gained EmptyFaceAllowlistIsInvalid cases for AllowAll and Query entries.
status: addressed
---
