<script setup vapor lang="ts">
const props = defineProps<{
  network: string
  results: readonly { value: string; label: string; detail: string; disabled?: boolean }[]
  selected: readonly { value: string; label: string }[]
  loading: boolean
  busy: boolean
  searchError: string
  error: string
  locked?: string
  blocked?: boolean
  maxNameLength: number
  maxDescriptionLength: number
  minimum: number
}>()
const emit = defineEmits<{
  close: []
  search: [query: string | null]
  add: [value: string]
  remove: [value: string]
  create: [name: string, description: string]
}>()
const { t } = useI18n()
const name = ref('')
const description = ref('')
</script>

<template>
  <AppDialog
    :title="t('workspaceUi.party.createTitle')"
    @close="emit('close')"
  >
    <form
      class="grid gap-4 px-5 pb-5"
      @submit.prevent="emit('create', name.trim(), description.trim())"
    >
      <SettingsDescription>{{
        t('workspaceUi.party.createDescription', { network: props.network })
      }}</SettingsDescription>
      <PartyPeoplePicker
        :results="results"
        :selected="selected"
        :loading="loading"
        :error="searchError"
        :locked="locked"
        @search="emit('search', $event)"
        @add="emit('add', $event)"
        @remove="emit('remove', $event)"
      />
      <AppField
        id="party-name"
        :label="t('workspaceUi.party.nameOptional')"
      >
        <template #default="control"
          ><UiInput
            v-bind="control"
            v-model="name"
            :maxlength="maxNameLength"
        /></template>
      </AppField>
      <AppField
        id="party-description"
        :label="t('workspaceUi.party.descriptionOptional')"
      >
        <template #default="control"
          ><UiTextarea
            v-bind="control"
            v-model="description"
            rows="3"
            :maxlength="maxDescriptionLength"
        /></template>
      </AppField>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        role="alert"
        >{{ error }}</UiAlert
      >
      <div class="flex justify-end gap-2">
        <UiButton
          type="button"
          variant="secondary"
          @click="emit('close')"
          >{{ t('common.cancel') }}</UiButton
        >
        <UiButton
          type="submit"
          :disabled="busy || blocked || selected.length < minimum"
          >{{ busy ? t('workspaceUi.party.creating') : t('workspaceUi.party.create') }}</UiButton
        >
      </div>
    </form>
  </AppDialog>
</template>
