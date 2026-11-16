<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'
import {
  guideKey,
  initialGuideSelection,
  type GuideChannel,
} from '../../../shared/utils/channel-guide'

type AdminGuide = components['schemas']['AdminChannelGuide']
type Entry = components['schemas']['ChannelGuideEntry']
type Request = components['schemas']['ChannelGuideRequestView']
type Directory = components['schemas']['GuideDirectoryChannel']
type State = Entry['state']

/**
 * Administration → Welcome (#258): the channel guide new members pick from when they join and
 * everyone can browse in Join existing. Owners edit groups and channels here, see a preview,
 * and approve, edit or decline channel operators' requests. Saving never joins or parts anyone.
 */
const MAX_GROUPS = 12
const MAX_ENTRIES = 50
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const refused = useAdministrationRefusal()
const { say } = useStatusAnnouncer()
const { full } = useTimeFormat()

const loaded = ref<AdminGuide>()
const groups = ref<string[]>([])
const entries = ref<Entry[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref<unknown>()
const conflict = ref(false)
const notice = ref('')
watch(notice, (text) => text && say(text))

function adopt(value: AdminGuide) {
  loaded.value = value
  groups.value = value.guide.groups.map((g) => g.name)
  entries.value = value.guide.entries.map((e) => ({
    ...e,
    group: e.group ?? '',
    description: e.description ?? '',
  }))
}
const draft = computed(() => ({
  groups: groups.value.map((name) => ({ name: name.trim() })),
  entries: entries.value.map((e) => ({
    channel: e.channel,
    group: e.group || undefined,
    description: e.description?.trim() || undefined,
    state: e.state,
  })),
}))
const savedDraft = computed(() =>
  loaded.value
    ? JSON.stringify({
        groups: loaded.value.guide.groups,
        entries: loaded.value.guide.entries.map((e) => ({
          channel: e.channel,
          group: e.group || undefined,
          description: e.description || undefined,
          state: e.state,
        })),
      })
    : '',
)
const dirty = computed(() => !!loaded.value && JSON.stringify(draft.value) !== savedDraft.value)

async function load() {
  loading.value = true
  error.value = undefined
  conflict.value = false
  try {
    adopt(await apiClient().administrationChannelGuide())
  } catch (e) {
    refused(e)
    error.value = e
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save(done = t('channelGuide.admin.saved')) {
  if (!loaded.value || saving.value) return false
  saving.value = true
  error.value = undefined
  conflict.value = false
  notice.value = ''
  try {
    adopt(
      await apiClient().saveChannelGuide({ revision: loaded.value.revision, guide: draft.value }),
    )
    notice.value = done
    return true
  } catch (e) {
    refused(e)
    if (errorStatus(e) === 409) conflict.value = true
    error.value = e
    return false
  } finally {
    saving.value = false
  }
}

// Groups
const newGroup = ref('')
function addGroup() {
  const name = newGroup.value.trim()
  if (!name || groups.value.some((g) => g.toLowerCase() === name.toLowerCase())) return
  groups.value.push(name)
  newGroup.value = ''
}
function renameGroup(index: number, name: string) {
  const before = groups.value[index]
  groups.value[index] = name
  for (const e of entries.value) if (e.group === before) e.group = name
}
function removeGroup(index: number) {
  const [name] = groups.value.splice(index, 1)
  for (const e of entries.value) if (e.group === name) e.group = ''
}
function move<T>(list: T[], index: number, by: number) {
  const to = index + by
  if (to < 0 || to >= list.length) return
  const [item] = list.splice(index, 1)
  list.splice(to, 0, item as T)
}

// Channels
const channelQuery = ref('')
const directory = ref<Directory[]>([])
let directoryTimer: ReturnType<typeof setTimeout> | undefined
let directoryController: AbortController | undefined
watch(channelQuery, (value) => {
  clearTimeout(directoryTimer)
  if (!loaded.value?.directory || !value.trim()) {
    directory.value = []
    return
  }
  directoryTimer = setTimeout(async () => {
    directoryController?.abort()
    directoryController = new AbortController()
    try {
      const page = await apiClient().guideDirectory(value.trim(), directoryController.signal)
      directory.value = page.items
    } catch (e) {
      if ((e as Error).name !== 'AbortError') refused(e)
    }
  }, 250)
})
onBeforeUnmount(() => {
  clearTimeout(directoryTimer)
  directoryController?.abort()
})
const listed = computed(() => new Set(entries.value.map((e) => guideKey(e.channel))))
const normalChannel = (value: string) => {
  const name = value.trim()
  return name.startsWith('#') ? name : `#${name}`
}
function addChannel(name: string, extra: Partial<Entry> = {}) {
  const channel = normalChannel(name)
  if (channel.length < 2 || listed.value.has(guideKey(channel))) return false
  if (entries.value.length >= MAX_ENTRIES) {
    notice.value = t('channelGuide.admin.tooManyChannels', { count: n(MAX_ENTRIES) })
    return false
  }
  entries.value.push({ channel, group: '', description: '', state: 'listed', ...extra })
  channelQuery.value = ''
  return true
}
const warnings = computed(() => {
  const out = new Map<string, string>()
  for (const w of loaded.value?.warnings ?? []) out.set(guideKey(w.channel), w.kind)
  return out
})
const warningText = (kind?: string) =>
  kind === 'missing'
    ? t('channelGuide.admin.warningMissing')
    : kind === 'hidden'
      ? t('channelGuide.admin.warningHidden')
      : kind === 'unregistered'
        ? t('channelGuide.admin.warningUnregistered')
        : ''
const stateOptions = computed(() => [
  { value: 'everyone' as State, label: t('channelGuide.forEveryone') },
  { value: 'suggested' as State, label: t('channelGuide.admin.suggested') },
  { value: 'listed' as State, label: t('channelGuide.admin.listed') },
])
const groupOptions = computed(() => [
  { value: '', label: t('channelGuide.moreChannels') },
  ...groups.value.filter((g) => g.trim()).map((g) => ({ value: g, label: g })),
])

// Requests from channel operators
function applyRequest(r: Request) {
  let group = ''
  if (r.group) {
    group = groups.value.find((g) => g.toLowerCase() === r.group!.toLowerCase()) ?? ''
    if (!group && groups.value.length < MAX_GROUPS) {
      groups.value.push(r.group)
      group = r.group
    }
  }
  return addChannel(r.channel, { group, description: r.description ?? '' })
}
async function approve(r: Request) {
  if (!applyRequest(r)) return
  await save(t('channelGuide.admin.approved', { channel: r.channel }))
}
function edit(r: Request) {
  if (applyRequest(r)) notice.value = t('channelGuide.admin.editing', { channel: r.channel })
}
const declining = ref('')
async function decline(r: Request) {
  declining.value = r.channel
  error.value = undefined
  try {
    await apiClient().declineGuideRequest(r.channel)
    if (loaded.value)
      loaded.value.requests = loaded.value.requests.filter((x) => x.channel !== r.channel)
    notice.value = t('channelGuide.admin.declined', { channel: r.channel })
  } catch (e) {
    refused(e)
    error.value = e
  } finally {
    declining.value = ''
  }
}
const pending = computed(() =>
  (loaded.value?.requests ?? []).filter((r) => !listed.value.has(guideKey(r.channel))),
)

// Preview, as a new member sees it
const previewChannels = computed<GuideChannel[]>(() =>
  entries.value.map((e) => ({
    channel: normalChannel(e.channel),
    group: e.group || undefined,
    description: e.description?.trim() || undefined,
    state: e.state,
    nsfw: false,
  })),
)
const previewGroups = computed(() => draft.value.groups.filter((g) => g.name))
const previewPicked = ref<string[]>([])
watch(previewChannels, (value) => (previewPicked.value = initialGuideSelection(value, new Set())), {
  immediate: true,
})
</script>
<template>
  <div class="admin-page admin-welcome grid gap-7">
    <SettingsDescription>{{ t('channelGuide.admin.intro') }}</SettingsDescription>
    <FieldHint v-if="loading && !loaded">{{ t('channelGuide.admin.loading') }}</FieldHint>
    <UiAlert
      v-if="error"
      as="div"
      variant="destructive"
      presentation="inline"
      class="form-error"
    >
      <p class="m-0">
        {{ conflict ? t('channelGuide.admin.conflict') : errorMessage(error) }}
      </p>
      <UiButton
        v-if="conflict"
        variant="link"
        size="text"
        @click="load"
        >{{ t('channelGuide.admin.reload') }}</UiButton
      >
    </UiAlert>
    <template v-if="loaded">
      <SettingGroup
        v-if="pending.length"
        :title="t('channelGuide.admin.requests')"
      >
        <SettingsDescription>{{ t('channelGuide.admin.requestsHelp') }}</SettingsDescription>
        <ul class="guide-requests list-none m-0 p-0 grid gap-2">
          <li
            v-for="r in pending"
            :key="r.channel"
            class="grid gap-2 rounded-[var(--radius-large)] bg-card px-3.5 py-3 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
            :data-channel="r.channel"
          >
            <div class="grid gap-0.5 min-w-0">
              <strong class="text-foreground"
                >{{ r.channel
                }}<span
                  v-if="r.members !== undefined"
                  class="ml-2 font-[400] text-[12px] text-faint"
                  >{{ t('join.userCount', { count: n(r.members) }, r.members) }}</span
                ></strong
              >
              <small class="text-[12px] text-muted-foreground">{{
                t('channelGuide.admin.requestedBy', {
                  account: r.requester,
                  time: full(r.createdAt),
                })
              }}</small>
              <p class="m-0 text-[13px] text-body">
                <span
                  v-if="r.group"
                  class="mr-1.5 py-[1px] px-1.5 rounded-[4px] bg-selection text-primary text-[11px] font-[600]"
                  >{{ r.group }}</span
                >{{ r.description || r.topic || t('channelGuide.admin.noDescription') }}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <UiButton
                size="sm"
                :disabled="saving || !!declining"
                @click="approve(r)"
                >{{ t('channelGuide.admin.approve') }}</UiButton
              ><UiButton
                size="sm"
                variant="secondary"
                :disabled="saving || !!declining"
                @click="edit(r)"
                >{{ t('channelGuide.admin.edit') }}</UiButton
              ><UiButton
                size="sm"
                variant="ghost"
                :disabled="saving || !!declining"
                @click="decline(r)"
                >{{
                  declining === r.channel
                    ? t('channelGuide.admin.declining')
                    : t('channelGuide.admin.decline')
                }}</UiButton
              >
            </div>
          </li>
        </ul>
      </SettingGroup>

      <SettingGroup :title="t('channelGuide.admin.groups')">
        <SettingsDescription>{{ t('channelGuide.admin.groupsHelp') }}</SettingsDescription>
        <ul class="guide-groups list-none m-0 p-0 grid gap-1.5">
          <li
            v-for="(g, index) in groups"
            :key="index"
            class="flex items-center gap-1.5"
          >
            <UiInput
              :model-value="g"
              class="flex-1"
              maxlength="40"
              :aria-label="t('channelGuide.admin.groupName')"
              @update:model-value="renameGroup(index, String($event))"
            /><UiButton
              variant="ghost"
              size="icon-sm"
              :disabled="index === 0"
              :aria-label="t('channelGuide.admin.moveUp', { name: g })"
              @click="move(groups, index, -1)"
              ><AppIcon
                name="moveUp"
                :size="15" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              :disabled="index === groups.length - 1"
              :aria-label="t('channelGuide.admin.moveDown', { name: g })"
              @click="move(groups, index, 1)"
              ><AppIcon
                name="moveDown"
                :size="15" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              :aria-label="t('channelGuide.admin.removeGroup', { name: g })"
              @click="removeGroup(index)"
              ><AppIcon
                name="close"
                :size="15"
            /></UiButton>
          </li>
        </ul>
        <form
          class="flex items-center gap-2 mt-2"
          @submit.prevent="addGroup"
        >
          <UiInput
            v-model="newGroup"
            class="flex-1"
            maxlength="40"
            :placeholder="t('channelGuide.admin.newGroup')"
            :aria-label="t('channelGuide.admin.newGroup')"
          /><UiButton
            variant="secondary"
            :disabled="!newGroup.trim() || groups.length >= MAX_GROUPS"
            >{{ t('channelGuide.admin.addGroup') }}</UiButton
          >
        </form>
      </SettingGroup>

      <SettingGroup :title="t('channelGuide.admin.channels')">
        <SettingsDescription>{{ t('channelGuide.admin.channelsHelp') }}</SettingsDescription>
        <p
          v-if="!loaded.saved"
          class="guide-defaults m-0 mb-2 text-[13px] text-muted-foreground"
        >
          {{
            loaded.defaults.length
              ? t('channelGuide.admin.notSavedDefaults', {
                  channels: loaded.defaults.join(', '),
                })
              : t('channelGuide.admin.notSaved')
          }}
        </p>
        <ul class="guide-entries list-none m-0 p-0 grid gap-2">
          <li
            v-for="(e, index) in entries"
            :key="e.channel"
            class="grid gap-2 rounded-[var(--radius-large)] bg-card px-3.5 py-3 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
            :data-channel="e.channel"
          >
            <div class="flex items-center gap-1.5 min-w-0">
              <strong class="flex-1 truncate text-foreground">{{ e.channel }}</strong>
              <UiButton
                variant="ghost"
                size="icon-sm"
                :disabled="index === 0"
                :aria-label="t('channelGuide.admin.moveUp', { name: e.channel })"
                @click="move(entries, index, -1)"
                ><AppIcon
                  name="moveUp"
                  :size="15" /></UiButton
              ><UiButton
                variant="ghost"
                size="icon-sm"
                :disabled="index === entries.length - 1"
                :aria-label="t('channelGuide.admin.moveDown', { name: e.channel })"
                @click="move(entries, index, 1)"
                ><AppIcon
                  name="moveDown"
                  :size="15" /></UiButton
              ><UiButton
                variant="ghost"
                size="icon-sm"
                :aria-label="t('channelGuide.admin.removeChannel', { name: e.channel })"
                @click="entries.splice(index, 1)"
                ><AppIcon
                  name="close"
                  :size="15"
              /></UiButton>
            </div>
            <p
              v-if="warnings.get(guideKey(e.channel))"
              class="guide-warning m-0 flex items-center gap-1.5 text-[12px] text-warning"
            >
              <AppIcon
                name="alert"
                :size="13"
              />{{ warningText(warnings.get(guideKey(e.channel))) }}
            </p>
            <div class="flex flex-wrap items-center gap-2">
              <AppSelect
                v-model="e.state"
                :options="stateOptions"
                class="w-auto flex-none"
                :aria-label="t('channelGuide.admin.stateFor', { channel: e.channel })"
              /><AppSelect
                v-model="e.group"
                :options="groupOptions"
                class="w-auto flex-none"
                :aria-label="t('channelGuide.admin.groupFor', { channel: e.channel })"
              />
            </div>
            <UiInput
              v-model="e.description"
              maxlength="120"
              :placeholder="t('channelGuide.admin.descriptionPlaceholder')"
              :aria-label="t('channelGuide.admin.descriptionFor', { channel: e.channel })"
            />
          </li>
        </ul>
        <form
          class="guide-add mt-2 grid gap-1.5"
          @submit.prevent="addChannel(channelQuery)"
        >
          <div class="flex items-center gap-2">
            <UiInput
              v-model="channelQuery"
              class="flex-1"
              maxlength="50"
              :placeholder="t('channelGuide.admin.addChannelPlaceholder')"
              :aria-label="t('channelGuide.admin.addChannel')"
            /><UiButton
              variant="secondary"
              :disabled="!channelQuery.trim() || entries.length >= MAX_ENTRIES"
              >{{ t('channelGuide.admin.add') }}</UiButton
            >
          </div>
          <div
            v-if="directory.length"
            class="guide-directory grid gap-0.5"
          >
            <button
              v-for="c in directory"
              :key="c.name"
              type="button"
              class="flex items-center gap-2 rounded-sm px-2 py-1.5 text-left text-[13px] text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
              :disabled="listed.has(guideKey(c.name))"
              @click="addChannel(c.name)"
            >
              <strong class="text-foreground">{{ c.name }}</strong
              ><span class="flex-1 truncate text-faint">{{ c.topic }}</span
              ><span class="text-[12px] text-faint">{{
                t('join.userCount', { count: n(c.users) }, c.users)
              }}</span>
            </button>
          </div>
        </form>
      </SettingGroup>

      <SettingGroup :title="t('channelGuide.admin.preview')">
        <SettingsDescription>{{ t('channelGuide.admin.previewHelp') }}</SettingsDescription>
        <div
          class="guide-preview rounded-[var(--radius-large)] bg-card p-2 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
        >
          <ChannelGuidePicker
            v-if="previewChannels.length"
            v-model="previewPicked"
            :groups="previewGroups"
            :channels="previewChannels"
            :joined="new Set()"
          />
          <FieldHint v-else>{{ t('channelGuide.admin.previewEmpty') }}</FieldHint>
        </div>
      </SettingGroup>

      <div
        class="guide-save sticky bottom-0 flex items-center justify-end gap-3 bg-background py-2"
      >
        <FieldHint
          v-if="loaded.updatedAt && !dirty"
          class="mr-auto"
          >{{
            t('channelGuide.admin.lastSaved', {
              account: loaded.updatedBy ?? '',
              time: full(loaded.updatedAt),
            })
          }}</FieldHint
        >
        <FieldHint
          v-if="notice"
          class="mr-auto"
          >{{ notice }}</FieldHint
        >
        <UiButton
          :disabled="(!dirty && loaded.saved) || saving"
          @click="save()"
          >{{ saving ? t('common.saving') : t('channelGuide.admin.save') }}</UiButton
        >
      </div>
    </template>
  </div>
</template>
