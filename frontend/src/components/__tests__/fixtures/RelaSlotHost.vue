<!--
  Fixture for RelaSlot.test.ts.

  Must be a real SFC: only build-time-compiled templates see the
  `isCustomElement` compilerOption that suppresses Vue's unresolved-component
  warning for `rela-` tags. A runtime string template would warn regardless.
-->
<template>
  <div class="rela-na" data-band="stalled">
    <rela-slot name="companion" data-band="stalled" />
  </div>
</template>
