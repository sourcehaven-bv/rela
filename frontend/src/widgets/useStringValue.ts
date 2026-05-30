import { computed, type ComputedRef } from 'vue'

// Renders a property value as a string for input/textarea/select binding.
// null and undefined become '' so the browser doesn't show "null"/"undefined";
// everything else gets String() coerced — same shape as FieldRenderer's
// historical stringValue helper.
export function useStringValue(source: () => unknown): ComputedRef<string> {
  return computed(() => {
    const v = source()
    if (v === null || v === undefined) return ''
    return String(v)
  })
}
