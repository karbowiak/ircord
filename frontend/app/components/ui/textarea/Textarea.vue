<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { useVModel } from '@vueuse/core'

const props = defineProps<{
  class?: HTMLAttributes['class']
  defaultValue?: string | number
  modelValue?: string | number
}>()

const emits = defineEmits<{
  (e: 'update:modelValue', payload: string | number): void
}>()

const modelValue = useVModel(props, 'modelValue', emits, {
  passive: true,
  defaultValue: props.defaultValue,
})
const field = ref<HTMLTextAreaElement | null>(null)
defineExpose({ focus: () => field.value?.focus() })
</script>

<template>
  <textarea
    ref="field"
    v-model="modelValue"
    data-slot="textarea"
    :class="
      cn(
        'w-full min-w-0 resize-y rounded-md border border-solid border-input bg-field px-[10px] py-2 text-[13px] leading-normal text-foreground placeholder:text-faint disabled:cursor-not-allowed disabled:opacity-55 read-only:bg-card hover:not-focus:not-disabled:border-[color-mix(in_srgb,var(--muted)_45%,var(--border))] focus:border-[color-mix(in_srgb,var(--accent)_60%,var(--border))] aria-invalid:border-destructive max-[700px]:text-[16px]',
        props.class,
      )
    "
  />
</template>
