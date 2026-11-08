<script setup lang="ts">
/**
 * A labelled text field: a native input (or textarea with `multiline`) inside AppField. Other
 * attributes — type, autocomplete, minlength, placeholder, name and the rest — go to the input,
 * so validation, autofill and password managers work as usual.
 */
defineOptions({ inheritAttrs: false })
withDefaults(
  defineProps<{
    label: string
    id?: string
    help?: string
    error?: string
    status?: string
    required?: boolean
    multiline?: boolean
  }>(),
  {
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
const field = ref<HTMLInputElement | HTMLTextAreaElement | null>(null)
defineExpose({ focus: () => field.value?.focus() })
</script>
<template>
  <AppField
    v-slot="control"
    :label="label"
    :id="id"
    :help="help"
    :error="error"
    :status="status"
    :required="required"
  >
    <textarea
      v-if="multiline"
      ref="field"
      v-model="model"
      class="text-input"
      v-bind="bind(control)"
    />
    <input
      v-else
      ref="field"
      v-model="model"
      class="text-input"
      v-bind="bind(control)"
    />
  </AppField>
</template>
