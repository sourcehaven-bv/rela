<script setup lang="ts">
/**
 * Not a component: a compile-time check that an overlay's `attrs` binding fits
 * both kinds of trigger.
 *
 * `RlMenu` and `RlPopover` hand the trigger slot an `attrs` object to spread,
 * and the two kinds of trigger constrain it in opposite directions. A native
 * `<button>` checks each value against the native attribute types, so a widened
 * `string` fails. A component's props are a closed set that admits no extra
 * keys and wants real booleans, so an object typed as "every native button
 * attribute" fails the other way. An object that satisfies one and not the
 * other typechecks in isolation and breaks at the call site.
 *
 * The stories cannot catch this. Their markup lives in string `template`
 * fields, which `vue-tsc` does not look inside, so the same four shapes written
 * there compile no matter what the binding's type is. Written here as real SFC
 * markup, they are checked on every build.
 */
import RlPopover from './RlPopover.vue'
import RlMenu from './RlMenu.vue'
import RlMenuItem from './RlMenuItem.vue'
import RlButton from '../common/RlButton.vue'
import RlIconButton from '../common/RlIconButton.vue'
</script>

<template>
  <!-- Native element triggers: the values must match native attribute types. -->
  <RlPopover title="Native">
    <template #trigger="{ toggle, attrs }">
      <button type="button" v-bind="attrs" @click="toggle">Open</button>
    </template>
    <p>Body</p>
  </RlPopover>

  <RlMenu>
    <template #trigger="{ toggle, attrs }">
      <button type="button" v-bind="attrs" @click="toggle">Menu</button>
    </template>
    <RlMenuItem>Item</RlMenuItem>
  </RlMenu>

  <!-- Component triggers: the object must carry no key the props lack. -->
  <RlMenu>
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" v-bind="attrs" @click="toggle">Export</RlButton>
    </template>
    <RlMenuItem>CSV</RlMenuItem>
  </RlMenu>

  <RlPopover title="Component">
    <template #trigger="{ toggle, attrs }">
      <RlIconButton icon="ellipsis" label="More" v-bind="attrs" @click="toggle" />
    </template>
    <p>Body</p>
  </RlPopover>
</template>
