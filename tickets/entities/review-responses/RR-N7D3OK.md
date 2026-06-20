---
id: RR-N7D3OK
type: review-response
title: Nested maps with non-string keys decode to map[any]any (fs) and diverge + break recursion
finding: 'REPRODUCED. yaml.v3 decodes a nested mapping with non-string keys (e.g. m:\n 1: a\n 2: b) to map[interface{}]interface{}, not map[string]any. The JSON path can never produce this (JSON keys are always strings → map[string]any). canonicalValue has no map[any]any case → fs hits default/fmt.Sprintf, pg hits the proper map[string]any case → divergent. Worse, map[any]any also BREAKS RECURSION: nested values inside it are never canonicalized through type-widening, so even string-keyed-but-typed-as-any maps lose invariance. Note entity.CloneValue (entity.go:184-205) also doesn''t handle map[any]any, so this type leaks through fsstore unchanged. FIX: handle map[any]any explicitly (convert keys via fmt.Sprint, recurse) OR normalize it out at the store boundary before canonical sees it.'
severity: critical
resolution: 'normalize() handles map[any]any explicitly: keys stringified via stringifyKey, values recursed, producing map[string]any — the same shape pgstore yields. Regression TestHashEntity_NonStringKeyedMap asserts map[any]any{1:a,2:b} == map[string]any{''1'':a,''2'':b}. Recursion is no longer broken because normalize recurses into every container before the writer sees it.'
status: addressed
---
