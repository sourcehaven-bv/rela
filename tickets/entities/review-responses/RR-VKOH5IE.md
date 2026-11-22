---
id: RR-VKOH5IE
type: review-response
title: Redundant not-exist check in loadForms
finding: stepforms.go:57 errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) is redundant; errors.Is alone suffices.
severity: nit
resolution: loadForms now checks only errors.Is(err, fs.ErrNotExist).
status: addressed
---
