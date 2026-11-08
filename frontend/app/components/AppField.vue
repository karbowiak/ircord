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
    class="app-field grid gap-1.5"
    :class="{ invalid: !!error }"
  >
    <UiLabel
      :id="labelId"
      class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
      :for="controlId"
      >{{ label }}</UiLabel
    >
    <slot v-bind="control" />
    <p
      v-if="help"
      :id="helpId"
      class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
    >
      {{ help }}
    </p>
    <p
      v-if="error"
      :id="errorId"
      class="form-error text-destructive text-[13px] mt-3"
      role="alert"
    >
      {{ error }}
    </p>
    <p
      v-else-if="status"
      class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5 field-status"
      role="status"
    >
      {{ status }}
    </p>
  </div>
</template>
