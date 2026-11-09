<script setup lang="ts">
/**
 * A labelled text field: a native input (or textarea with `multiline`) inside AppField. Other
 * attributes — type, autocomplete, minlength, placeholder, name and the rest — go to the input,
 * so validation, autofill and password managers work as usual.
 */
defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    /** Or the `label` slot, for a label with markup. */
    label?: string
    id?: string
    help?: string
    error?: string
    status?: string
    required?: boolean
    multiline?: boolean
  }>(),
  {
    label: undefined,
    id: undefined,
    help: undefined,
    error: undefined,
    status: undefined,
    required: false,
    multiline: false,
  },
)
const model = defineModel<string>({ default: '' })
const attrs = useAttrs()
/** The field's ids plus any description the caller adds, rather than one replacing the other. */
const bind = (control: Record<string, unknown>) => ({
  ...attrs,
  ...control,
  'aria-describedby':
    [attrs['aria-describedby'], control['aria-describedby']].filter(Boolean).join(' ') || undefined,
})
const field = ref<{ focus: () => void } | null>(null)
defineExpose({ focus: () => field.value?.focus() })
</script>
<template>
  <AppField
    :id="id"
    :label="label"
    :help="help"
    :error="error"
    :status="status"
    :required="required"
  >
    <template
      v-if="$slots.label"
      #label
      ><slot name="label"
    /></template>
    <template #default="control">
      <UiTextarea
        v-if="multiline"
        ref="field"
        :model-value="model"
        v-bind="bind(control)"
        @update:model-value="model = String($event)"
      />
      <UiInput
        v-else
        ref="field"
        :model-value="model"
        v-bind="bind(control)"
        @update:model-value="model = String($event)"
      />
    </template>
  </AppField>
</template>
