---
id: RR-G3AD6
type: review-response
title: Use Vue idiom (update:modelValue + commit emit), not React onChange prop
finding: 'Vue 3 standard is defineEmits + v-model. FieldRenderer already uses emit(''update'', ...). RruleBuilder/TagSelect already use update:model-value. onChange callback breaks v-model, DevTools event tracing, and modifiers like .lazy/.trim. Also makes async error handling weird (which is what open-question #2 was about).'
severity: significant
resolution: 'Plan revised: widgets use defineEmits<{''update:modelValue'':[T], commit?:[T]}>() (Vue idiom). v-model works; DevTools tracing works. Persistence state flows down as props (disabled/error) — open-question #2 dissolves. See TKT-MZSIJ ''Widget component shape''.'
status: addressed
---
