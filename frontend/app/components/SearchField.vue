<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/**
 * A search or filter box: a magnifier, a borderless input and an optional key hint. `lg` sits in
 * a page toolbar, as tall as the selects and buttons beside it; `sm` filters a list. Other attributes and listeners (aria-label, placeholder,
 * autofocus, keydown…) go to the input; `class` styles the box.
 */
defineOptions({ inheritAttrs: false })
const props = withDefaults(
  defineProps<{ size?: 'lg' | 'sm'; hint?: string; class?: HTMLAttributes['class'] }>(),
  { size: 'sm', hint: undefined, class: undefined },
)
const model = defineModel<string>({ default: '' })
const { t } = useI18n()
const input = ref<HTMLInputElement | null>(null)
function clear() {
  model.value = ''
  input.value?.focus()
}
defineExpose({ focus: () => input.value?.focus() })
</script>
<template>
  <div
    :class="
      cn(
        'search-field-control flex min-w-0 items-center border border-solid border-border bg-field text-muted-foreground transition-colors focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/15',
        size === 'lg' ? 'h-9 gap-2.5 rounded-md px-3' : 'h-8 gap-2 rounded-md px-2.5',
        props.class,
      )
    "
  >
    <AppIcon
      name="search"
      :size="size === 'lg' ? 16 : 14"
    />
    <input
      ref="input"
      v-model="model"
      v-bind="$attrs"
      :class="
        cn(
          'min-w-0 flex-1 border-0 bg-transparent text-foreground outline-none placeholder:text-faint max-[700px]:text-[16px] [@media(pointer:coarse)]:text-[length:max(16px,var(--ui-small))]',
          size === 'lg' ? 'text-[length:var(--ui-text)]' : 'text-[length:var(--ui-small)]',
        )
      "
    />
    <button
      v-if="model"
      type="button"
      class="flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground"
      :aria-label="t('searchField.clear')"
      @click="clear"
    >
      <AppIcon
        name="close"
        :size="14"
      />
    </button>
    <kbd v-if="hint">{{ hint }}</kbd>
  </div>
</template>
