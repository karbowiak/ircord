<script setup vapor lang="ts">
/**
 * A list of short values as removable chips, with a field that adds one on Enter. `add` returns
 * false to keep the typed text (for example when saving failed). Other attributes go to the field.
 */
defineOptions({ inheritAttrs: false })
const props = defineProps<{
  items: readonly string[]
  add: (value: string) => unknown
  remove: (value: string) => void
  /** The remove button's accessible name for one chip. */
  removeLabel: (value: string) => string
}>()
const text = ref('')
function submit() {
  const value = text.value.trim()
  if (value && props.add(value) === false) return
  text.value = ''
}
</script>
<template>
  <form
    class="chip-input flex min-h-10 flex-wrap gap-1.5 rounded-md bg-field p-1.5 shadow-[inset_0_0_0_1px_var(--border)] focus-within:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_60%,var(--border))]"
    @submit.prevent="submit"
  >
    <span
      v-for="item in items"
      :key="item"
      class="chip inline-flex h-6.5 items-center gap-1 rounded-[13px] bg-secondary pr-1 pl-2.5 text-[13px]"
      >{{ item
      }}<UiButton
        variant="ghost"
        size="icon-xs"
        class="size-5 rounded-full"
        type="button"
        :aria-label="removeLabel(item)"
        @click="remove(item)"
      >
        <AppIcon
          name="close"
          :size="12"
        /> </UiButton
    ></span>
    <input
      v-model="text"
      v-bind="$attrs"
      class="min-w-40 flex-1 border-0 bg-transparent px-1.5 text-[13px] outline-none max-[700px]:text-[16px]"
    />
  </form>
</template>
