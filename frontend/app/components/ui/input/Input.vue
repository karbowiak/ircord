<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { useVModel } from '@vueuse/core'

const props = defineProps<{
  defaultValue?: string | number
  modelValue?: string | number
  class?: HTMLAttributes['class']
}>()

const emits = defineEmits<{
  (e: 'update:modelValue', payload: string | number): void
}>()

const modelValue = useVModel(props, 'modelValue', emits, {
  passive: true,
  defaultValue: props.defaultValue,
})
const field = ref<HTMLInputElement | null>(null)
defineExpose({
  focus: () => field.value?.focus(),
  setCustomValidity: (message: string) => field.value?.setCustomValidity(message),
})
</script>

<template>
  <input
    ref="field"
    v-model="modelValue"
    data-slot="input"
    :class="
      cn(
        'w-full min-w-0 rounded-md border border-solid border-control bg-field px-[10px] py-2 text-[length:var(--ui-small)] leading-normal text-foreground placeholder:text-faint disabled:cursor-not-allowed disabled:opacity-55 read-only:bg-card hover:not-focus:not-disabled:border-muted-foreground focus:border-[color-mix(in_srgb,var(--accent)_60%,var(--control-border))] aria-invalid:border-destructive max-[700px]:text-[16px] [@media(pointer:coarse)]:text-[length:max(16px,var(--ui-small))]',
        props.class,
      )
    "
  >
</template>
