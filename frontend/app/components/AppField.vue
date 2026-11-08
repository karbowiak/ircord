<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/**
 * Label, help and error around one form control. The default slot receives the props to bind
 * on the control: `id`, `aria-labelledby`, `aria-describedby`, `aria-invalid` and `required`.
 * The `label` and `help` slots take rich content in place of the props; `helpLive` announces
 * help that changes as the user types (an availability check), and `helpClass` styles it.
 */
const props = withDefaults(
  defineProps<{
    label?: string
    id?: string
    help?: string
    error?: string
    /** A status line shown in place of help, such as “Saved”. */
    status?: string
    required?: boolean
    helpLive?: boolean
    helpClass?: HTMLAttributes['class']
  }>(),
  {
    label: undefined,
    id: undefined,
    help: undefined,
    error: undefined,
    status: undefined,
    required: false,
    helpLive: false,
    helpClass: undefined,
  },
)
const slots = useSlots()
const generated = useId()
const controlId = computed(() => props.id ?? generated)
const labelId = computed(() => `${controlId.value}-label`)
const helpId = computed(() => `${controlId.value}-help`)
const errorId = computed(() => `${controlId.value}-error`)
const hasHelp = computed(() => !!props.help || !!slots.help)
const control = computed(() => ({
  id: controlId.value,
  'aria-labelledby': labelId.value,
  'aria-describedby':
    [hasHelp.value && helpId.value, props.error && errorId.value].filter(Boolean).join(' ') ||
    undefined,
  'aria-invalid': props.error ? true : undefined,
  required: props.required || undefined,
}))
</script>
<template>
  <div
    class="app-field grid gap-1.5"
    :class="{ invalid: !!error }"
  >
    <FieldLabel
      :id="labelId"
      :for="controlId"
      ><slot name="label">{{ label }}</slot></FieldLabel
    >
    <slot v-bind="control" />
    <FieldHint
      v-if="hasHelp"
      :id="helpId"
      :class="helpClass"
      :aria-live="helpLive ? 'polite' : undefined"
    >
      <slot name="help">{{ help }}</slot>
    </FieldHint>
    <UiAlert
      as="p"
      variant="destructive"
      presentation="inline"
      v-if="error"
      :id="errorId"
      class="form-error mt-3"
    >
      {{ error }}
    </UiAlert>
    <FieldHint
      v-else-if="status"
      class="field-status"
      role="status"
    >
      {{ status }}
    </FieldHint>
  </div>
</template>
