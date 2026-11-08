<script setup lang="ts">
/**
 * Label, help and error around one form control. The default slot receives the props to bind
 * on the control: `id`, `aria-labelledby`, `aria-describedby`, `aria-invalid` and `required`.
 */
const props = withDefaults(
  defineProps<{
    label: string
    id?: string
    help?: string
    error?: string
    /** A status line shown in place of help, such as “Saved”. */
    status?: string
    required?: boolean
  }>(),
  { id: undefined, help: undefined, error: undefined, status: undefined, required: false },
)
const generated = useId()
const controlId = computed(() => props.id ?? generated)
const labelId = computed(() => `${controlId.value}-label`)
const helpId = computed(() => `${controlId.value}-help`)
const errorId = computed(() => `${controlId.value}-error`)
const control = computed(() => ({
  id: controlId.value,
  'aria-labelledby': labelId.value,
  'aria-describedby':
    [props.help && helpId.value, props.error && errorId.value].filter(Boolean).join(' ') ||
    undefined,
  'aria-invalid': props.error ? true : undefined,
  required: props.required || undefined,
}))
</script>
<template>
  <div
    class="app-field"
    :class="{ invalid: !!error }"
  >
    <label
      :id="labelId"
      class="field-label"
      :for="controlId"
      >{{ label }}</label
    >
    <slot v-bind="control" />
    <p
      v-if="help"
      :id="helpId"
      class="field-hint"
    >
      {{ help }}
    </p>
    <p
      v-if="error"
      :id="errorId"
      class="form-error"
      role="alert"
    >
      {{ error }}
    </p>
    <p
      v-else-if="status"
      class="field-hint field-status"
      role="status"
    >
      {{ status }}
    </p>
  </div>
</template>
