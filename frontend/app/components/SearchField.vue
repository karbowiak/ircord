<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/**
 * A search or filter box: a magnifier, a borderless input and an optional key hint. `lg` heads a
 * search dialog; `sm` filters a list. Other attributes and listeners (aria-label, placeholder,
 * autofocus, keydown…) go to the input; `class` styles the box.
 */
defineOptions({ inheritAttrs: false })
const props = withDefaults(
  defineProps<{ size?: 'lg' | 'sm'; hint?: string; class?: HTMLAttributes['class'] }>(),
  { size: 'sm', hint: undefined, class: undefined },
)
const model = defineModel<string>({ default: '' })
const input = ref<HTMLInputElement | null>(null)
defineExpose({ focus: () => input.value?.focus() })
</script>
<template>
  <label
    :class="
      cn(
        'flex min-w-0 items-center bg-rail text-faint',
        size === 'lg'
          ? 'gap-2.5 rounded-md px-3 py-2.5 focus-within:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_50%,transparent)]'
          : 'h-7.5 gap-1.5 rounded-sm px-2',
        props.class,
      )
    "
  >
    <AppIcon
      name="search"
      :size="size === 'lg' ? 21 : 14"
    />
    <input
      ref="input"
      v-model="model"
      v-bind="$attrs"
      :class="
        cn(
          'min-w-0 flex-1 border-0 bg-transparent text-foreground outline-none placeholder:text-faint max-[700px]:text-[16px]',
          size === 'lg' ? 'text-[15px]' : 'text-[13px]',
        )
      "
    />
    <kbd v-if="hint">{{ hint }}</kbd>
  </label>
</template>
