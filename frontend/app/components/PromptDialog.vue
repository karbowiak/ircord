<script setup lang="ts">
const { current, settle } = usePrompt()
const value = ref('')
const input = ref<HTMLInputElement | null>(null)
watch(
  current,
  (request) => {
    value.value = request?.value ?? ''
    if (request?.kind === 'prompt') nextTick(() => input.value?.select())
  },
  { immediate: true },
)
const blocked = computed(
  () =>
    (!!current.value?.requireText && value.value !== current.value.requireText) ||
    (current.value?.kind === 'prompt' &&
      !current.value.requireText &&
      !current.value.allowEmpty &&
      !value.value.trim()),
)
function submit() {
  if (!blocked.value) settle(current.value?.kind === 'confirm' ? '' : value.value.trim())
}
</script>
<template>
  <AppDialog
    v-if="current"
    :title="current.title"
    @close="settle(null)"
  >
    <form
      class="dialog-form pt-0 pr-5 pb-5 pl-5"
      @submit.prevent="submit"
    >
      <SettingsDescription v-if="current.body">
        {{ current.body }}
      </SettingsDescription>
      <AppField
        v-if="current.kind === 'prompt' || current.requireText"
        id="prompt-input"
        class="mt-4"
        :label="current.requireText ? `Type “${current.requireText}” to confirm` : current.label"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            :ref="(el) => (input = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
            v-model="value"
            :placeholder="current.placeholder"
            autocomplete="off"
            autofocus
          />
        </template>
      </AppField>
      <div class="form-actions flex justify-end items-center gap-2 mt-5">
        <UiButton
          variant="secondary"
          type="button"
          @click="settle(null)"
        >
          Cancel
        </UiButton>
        <UiButton
          :variant="current.danger ? 'destructive' : 'default'"
          :disabled="blocked"
        >
          {{ current.confirm }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
