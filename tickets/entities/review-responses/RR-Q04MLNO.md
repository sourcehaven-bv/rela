---
id: RR-Q04MLNO
type: review-response
title: Changed properties compare Go types, not values
finding: reflect.DeepEqual saw int 1 and float64 1 as different, so a no-op edit counted as a change.
severity: significant
resolution: sameValue falls back to comparing JSON encodings; TestChangedProps_TypeMismatch pins it.
status: addressed
---
