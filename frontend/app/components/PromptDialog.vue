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
      class="dialog-form"
      @submit.prevent="submit"
    >
      <p
        v-if="current.body"
        class="settings-description"
      >
        {{ current.body }}
      </p>
      <template v-if="current.kind === 'prompt' || current.requireText">
        <label
          class="field-label"
          for="prompt-input"
          >{{
            current.requireText ? `Type “${current.requireText}” to confirm` : current.label
          }}</label
        >
        <input
          id="prompt-input"
          ref="input"
          v-model="value"
          class="text-input"
          :placeholder="current.placeholder"
          autocomplete="off"
          autofocus
        />
      </template>
      <div class="form-actions">
        <button
          type="button"
          class="button secondary"
          @click="settle(null)"
        >
          Cancel
        </button>
        <button
          class="button"
          :class="current.danger ? 'danger' : 'primary'"
          :disabled="blocked"
        >
          {{ current.confirm }}
        </button>
      </div>
    </form>
  </AppDialog>
</template>
