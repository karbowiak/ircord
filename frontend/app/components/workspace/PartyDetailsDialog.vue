<script setup vapor lang="ts">
const props = defineProps<{
  title: string
  name: string
  description: string
  state: string
  members: readonly {
    value: string
    label: string
    detail: string
    avatarUrl?: string
    owner: boolean
    me: boolean
  }[]
  canManageMembers: boolean
  results: readonly { value: string; label: string; detail: string; disabled?: boolean }[]
  selected: readonly { value: string; label: string }[]
  loading: boolean
  busy: boolean
  searchError: string
  error: string
  maxNameLength: number
  maxDescriptionLength: number
  conversionStatus: string
  conversionWarnings: readonly string[]
}>()
const emit = defineEmits<{
  close: []
  save: [name: string, description: string]
  search: [query: string | null]
  add: [value: string]
  removeSelection: [value: string]
  invite: []
  removeMember: [value: string]
  transfer: [value: string]
  leave: []
  convert: []
}>()
const { t } = useI18n()
const name = ref(props.name)
const description = ref(props.description)
const changed = computed(() => name.value !== props.name || description.value !== props.description)
watch(
  () => [props.name, props.description],
  ([nextName, nextDescription], [oldName, oldDescription]) => {
    if (name.value === oldName && description.value === oldDescription) {
      name.value = nextName ?? ''
      description.value = nextDescription ?? ''
    }
  },
)
</script>

<template>
  <AppDialog
    :title="title"
    @close="emit('close')"
  >
    <div
      class="grid max-h-[min(70dvh,680px)] gap-5 overflow-y-auto px-5 pb-5 max-[700px]:max-h-none"
    >
      <p
        v-if="state !== 'active'"
        role="status"
        class="text-[length:var(--ui-small)] text-muted-foreground"
      >
        {{ t(`workspaceUi.party.state.${state}`) }}
      </p>
      <form
        class="grid gap-3"
        @submit.prevent="emit('save', name.trim(), description.trim())"
      >
        <AppField
          id="party-edit-name"
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
          id="party-edit-description"
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
        <div class="flex justify-end">
          <UiButton
            type="submit"
            size="sm"
            :disabled="busy || state === 'archived' || !changed"
            >{{ t('workspaceUi.party.save') }}</UiButton
          >
        </div>
      </form>
      <section
        class="grid gap-2"
        :aria-label="t('workspaceUi.party.people')"
      >
        <h3 class="text-[length:var(--ui-small)] font-semibold">
          {{ t('workspaceUi.party.people') }}
        </h3>
        <ul class="m-0 grid list-none gap-1 p-0">
          <li
            v-for="person in members"
            :key="person.value"
            class="flex min-w-0 items-center gap-2 py-1"
          >
            <UserAvatar
              :member="{ displayName: person.label, avatarUrl: person.avatarUrl }"
              size="small"
            />
            <span class="min-w-0 flex-1"
              ><span class="block truncate">{{ person.label }}</span
              ><span class="block truncate text-[length:var(--ui-caption)] text-muted-foreground"
                >{{ person.detail
                }}<template v-if="person.owner"> · {{ t('workspaceUi.party.owner') }}</template
                ><template v-if="person.me"> · {{ t('workspaceUi.party.you') }}</template></span
              ></span
            >
            <template v-if="canManageMembers && !person.me">
              <UiButton
                type="button"
                variant="ghost"
                size="sm"
                :disabled="busy || state === 'archived'"
                :aria-label="t('workspaceUi.party.transferTo', { name: person.label })"
                @click="emit('transfer', person.value)"
                >{{ t('workspaceUi.party.transfer') }}</UiButton
              >
              <UiButton
                type="button"
                variant="ghost"
                size="sm"
                :disabled="busy || state === 'archived'"
                :aria-label="t('workspaceUi.party.removePerson', { name: person.label })"
                @click="emit('removeMember', person.value)"
                >{{ t('workspaceUi.party.remove') }}</UiButton
              >
            </template>
          </li>
        </ul>
      </section>
      <section
        class="grid gap-2"
        :aria-label="t('workspaceUi.party.invitePeople')"
      >
        <h3 class="text-[length:var(--ui-small)] font-semibold">
          {{ t('workspaceUi.party.invitePeople') }}
        </h3>
        <PartyPeoplePicker
          :results="results"
          :selected="selected"
          :loading="loading"
          :error="searchError"
          @search="emit('search', $event)"
          @add="emit('add', $event)"
          @remove="emit('removeSelection', $event)"
        />
        <div class="flex justify-end">
          <UiButton
            size="sm"
            :disabled="busy || state === 'archived' || !selected.length"
            @click="emit('invite')"
            >{{ t('workspaceUi.party.invite') }}</UiButton
          >
        </div>
      </section>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        role="alert"
        >{{ error }}</UiAlert
      >
      <section
        v-if="canManageMembers && state !== 'archived'"
        class="grid gap-2"
      >
        <p class="text-[13px] text-muted-foreground">{{ t('workspaceUi.party.convertHelp') }}</p>
        <p
          v-if="conversionStatus"
          role="status"
          class="text-[13px]"
        >
          {{ conversionStatus }}
        </p>
        <ul
          v-if="conversionWarnings.length"
          class="list-disc pl-5 text-[13px] text-muted-foreground"
        >
          <li
            v-for="warning in conversionWarnings"
            :key="warning"
          >
            {{ warning }}
          </li>
        </ul>
        <div>
          <UiButton
            size="sm"
            variant="secondary"
            :disabled="busy"
            @click="emit('convert')"
            >{{ t('workspaceUi.party.convert') }}</UiButton
          >
        </div>
      </section>
      <div>
        <UiButton
          variant="destructive"
          size="sm"
          :disabled="busy || state === 'archived'"
          @click="emit('leave')"
          >{{ t('workspaceUi.party.leave') }}</UiButton
        >
      </div>
    </div>
  </AppDialog>
</template>
