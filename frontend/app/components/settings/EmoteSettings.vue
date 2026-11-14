<script setup vapor lang="ts">
import type { InstanceEmote } from '../../../shared/types/media'
import type { DiscoveryItem } from '../../composables/media/useEmoteDiscovery'
import { parseProviderEmote, providerName } from '../../../shared/utils/media'

const media = useMediaStore()
const chat = useChatStore()
const auth = useAuthStore()
const nameStyle = useNameStyle()
const queryCache = useQueryCache()
const { ask, confirm } = usePrompt()
const { t, n } = useI18n()
const $t = t
const $n = n
media.needCustom()
const catalog = computed(() => media.instanceQuery.data)
const catalogError = computed(() => media.instanceQuery.error)
const canManage = computed(() => !!catalog.value?.canManage)
const canAdd = computed(() => catalog.value?.canAdd ?? canManage.value)
const mayManage = (emote: InstanceEmote) => emote.canManage ?? canManage.value
const items = computed(() => catalog.value?.items ?? [])

const tab = ref<'server' | 'discover'>('server')
const panel = ref<'closed' | 'upload' | 'import' | 'discover'>('closed')
useViewHistory('emote-section', tab)
useViewHistory('emote-panel', panel)
const mode = computed(() =>
  panel.value === 'discover' ? 'discover' : panel.value === 'import' ? 'import' : 'upload',
)
function changeTab(value: unknown) {
  if ((value === 'server' || value === 'discover') && value !== tab.value) {
    panel.value = 'closed'
    tab.value = value
  }
}
const search = ref('')
const visibleItems = computed(() => {
  const query = search.value.trim().toLowerCase()
  return query
    ? items.value.filter((item) =>
        [item.shortcode, item.name, item.creator, item.source, ...(item.aliases ?? [])].some(
          (value) => value?.toLowerCase().includes(query),
        ),
      )
    : items.value
})
const file = ref<File>()
const previewUrl = ref('')
const stillUrl = ref('')
let previewGeneration = 0
/** Bumped to render a fresh, empty file input after an upload. */
const fileInput = ref(0)
const selected = ref<DiscoveryItem>()
const detailHeading = ref<HTMLElement>()
let detailTrigger: HTMLElement | null = null
// History may restore the detail after this component was remounted, when the selected
// provider item is no longer in memory. Return to the grid instead of opening an empty form.
watch(
  panel,
  (value) => {
    if (value === 'discover' && !selected.value) panel.value = 'closed'
  },
  { immediate: true, flush: 'sync' },
)
const drafts = reactive<Record<string, { shortcode: string; aliases: string }>>({
  upload: { shortcode: '', aliases: '' },
  import: { shortcode: '', aliases: '' },
  discover: { shortcode: '', aliases: '' },
})
const draftKey = computed(() =>
  mode.value === 'discover' && selected.value
    ? `${selected.value.provider}:${selected.value.providerId}`
    : mode.value,
)
const draft = computed(() => drafts[draftKey.value]!)
const shortcode = computed({
  get: () => draft.value.shortcode,
  set: (value: string) => {
    draft.value.shortcode = value
  },
})
const aliases = computed({
  get: () => draft.value.aliases,
  set: (value: string) => {
    draft.value.aliases = value
  },
})
const discovery = useEmoteDiscovery(computed(() => tab.value === 'discover'))
const {
  query: discoverQuery,
  source: discoverSource,
  animation: discoverAnimation,
  page: discoverPage,
  loading: discoverLoading,
  error: discoverError,
  cursors,
} = discovery
const apiRoot = (useRuntimeConfig().public.apiBase as string).replace(/\/api\/v1\/?$/, '')
const previewSource = (item: DiscoveryItem) => `${apiRoot}${item.previewUrl}`
const isInstalled = (item: DiscoveryItem) =>
  item.installed ||
  items.value.some(
    (emote) => emote.source === item.provider && emote.providerId === item.providerId,
  )
function selectDiscovery(item: DiscoveryItem, event: MouseEvent) {
  detailTrigger = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
  selected.value = item
  const key = `${item.provider}:${item.providerId}`
  drafts[key] ??= {
    shortcode: item.name
      .toLowerCase()
      .replace(/[^\w]+/g, '_')
      .slice(0, 32),
    aliases: '',
  }
  panel.value = 'discover'
  error.value = status.value = ''
  void nextTick(() => detailHeading.value?.focus({ preventScroll: true }))
}
function openUpload() {
  detailTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  panel.value = 'upload'
  void nextTick(() => document.getElementById('emote-file')?.focus())
}
function closeDetail() {
  panel.value = 'closed'
  void nextTick(() => {
    if (detailTrigger?.isConnected) detailTrigger.focus({ preventScroll: true })
    else
      document
        .getElementById(tab.value === 'server' ? 'emote-search' : 'discover-search')
        ?.focus({ preventScroll: true })
  })
}
const source = ref('')
const sourcePlaceholder = 'https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD'
const busy = ref(false)
const error = ref('')
const status = ref('')

async function refresh() {
  await queryCache.invalidateQueries({ key: ['emoji', 'instance'] })
}
async function run(action: () => Promise<unknown>, doneKey: string) {
  busy.value = true
  error.value = status.value = ''
  try {
    await action()
    status.value = doneKey
    try {
      await refresh()
    } catch {
      error.value = t('emotes.refreshFailed')
    }
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    busy.value = false
  }
}

function clearPreview() {
  previewGeneration++
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  if (stillUrl.value) URL.revokeObjectURL(stillUrl.value)
  previewUrl.value = stillUrl.value = ''
}
onScopeDispose(clearPreview)

async function selectFile(selected?: File) {
  if (!selected) return
  if (!catalog.value?.types?.includes(selected.type)) {
    error.value = t('emotes.invalidType')
    return
  }
  if (!selected.size || !catalog.value.maxBytes || selected.size > catalog.value.maxBytes) {
    error.value = t('emotes.invalidSize', { size: n(Math.round(catalog.value.maxBytes / 1024)) })
    return
  }
  clearPreview()
  error.value = ''
  file.value = selected
  previewUrl.value = URL.createObjectURL(selected)
  const generation = previewGeneration
  if (!shortcode.value)
    shortcode.value = selected.name
      .replace(/\.[^.]+$/, '')
      .replace(/[^\w]+/g, '_')
      .slice(0, 32)
  const still = await previewStill(selected)
  if (generation === previewGeneration && still) stillUrl.value = URL.createObjectURL(still)
}
function chooseFile(event: Event) {
  void selectFile((event.target as HTMLInputElement).files?.[0])
}
function dropFile(event: DragEvent) {
  void selectFile(event.dataTransfer?.files[0])
}

/**
 * The first frame, so viewers with animation off see a still. The server keeps it only for
 * animated images; it extracts a GIF's first frame itself, and JPEGs never animate.
 */
async function stillFrame(image: File): Promise<Blob | undefined> {
  if (image.type === 'image/jpeg' || image.type === 'image/gif') return undefined
  const url = URL.createObjectURL(image)
  try {
    // A detached element: decode() and naturalWidth feed the canvas, which NuxtImg cannot do.
    const element = new Image()
    element.src = url
    await element.decode()
    if (element.naturalWidth > 1024 || element.naturalHeight > 1024) return undefined
    const canvas = document.createElement('canvas')
    canvas.width = element.naturalWidth
    canvas.height = element.naturalHeight
    canvas.getContext('2d')?.drawImage(element, 0, 0)
    // WebP keeps the still small; browsers without a WebP encoder fall back to PNG.
    return await new Promise((resolve) =>
      canvas.toBlob((b) => resolve(b ?? undefined), 'image/webp', 0.9),
    )
  } catch {
    return undefined
  } finally {
    URL.revokeObjectURL(url)
  }
}

async function previewStill(image: File): Promise<Blob | undefined> {
  if (!image.type.startsWith('image/')) return undefined
  const url = URL.createObjectURL(image)
  try {
    const element = new Image()
    element.src = url
    await element.decode()
    if (element.naturalWidth > 1024 || element.naturalHeight > 1024) return undefined
    const canvas = document.createElement('canvas')
    canvas.width = element.naturalWidth
    canvas.height = element.naturalHeight
    canvas.getContext('2d')?.drawImage(element, 0, 0)
    return await new Promise((resolve) => canvas.toBlob((blob) => resolve(blob ?? undefined)))
  } catch {
    return undefined
  } finally {
    URL.revokeObjectURL(url)
  }
}

function add() {
  if (mode.value === 'upload') {
    const image = file.value
    if (!image) return
    const submittedDraft = drafts.upload!
    const submittedShortcode = shortcode.value
    const submittedAliases = aliases.value
    return run(async () => {
      await apiClient().uploadEmote({
        file: image,
        still: await stillFrame(image),
        shortcode: submittedShortcode,
        aliases: submittedAliases || undefined,
      })
      if (file.value === image) {
        file.value = undefined
        fileInput.value++
        clearPreview()
      }
      if (submittedDraft.shortcode === submittedShortcode) submittedDraft.shortcode = ''
      if (submittedDraft.aliases === submittedAliases) submittedDraft.aliases = ''
    }, 'emotes.added')
  }
  if (mode.value === 'discover') {
    const item = selected.value
    if (!item) return
    const submittedShortcode = shortcode.value
    const submittedAliases = aliases.value
    if (item.provider !== 'emoji.gg') return
    return run(async () => {
      await apiClient().importEmote({
        provider: 'emoji.gg',
        providerId: item.providerId,
        shortcode: submittedShortcode || undefined,
        aliases: submittedAliases
          .split(',')
          .map((a) => a.trim())
          .filter(Boolean),
      })
      item.installed = true
      discovery.refresh()
    }, 'emotes.imported')
  }
  const parsed = parseProviderEmote(source.value)
  if (!parsed) {
    error.value = t('emotes.pasteLink')
    return
  }
  const submittedSource = source.value
  const submittedDraft = drafts.import!
  const submittedShortcode = shortcode.value
  const submittedAliases = aliases.value
  return run(async () => {
    await apiClient().importEmote({
      ...parsed,
      shortcode: submittedShortcode || undefined,
      aliases: submittedAliases
        .split(',')
        .map((a) => a.trim())
        .filter(Boolean),
    })
    if (source.value === submittedSource) source.value = ''
    if (submittedDraft.shortcode === submittedShortcode) submittedDraft.shortcode = ''
    if (submittedDraft.aliases === submittedAliases) submittedDraft.aliases = ''
  }, 'emotes.imported')
}

async function rename(emote: InstanceEmote) {
  const next = await ask({
    title: t('emotes.rename.title', { shortcode: emote.shortcode }),
    body: t('emotes.rename.body'),
    label: t('emotes.shortcode'),
    value: emote.shortcode,
    confirm: t('emotes.rename.confirm'),
  })
  if (next && next !== emote.shortcode)
    await run(() => apiClient().updateEmote(emote.id, { shortcode: next }), 'emotes.renamed')
}
async function editAliases(emote: InstanceEmote) {
  const next = await ask({
    title: t('emotes.aliasesFor', { shortcode: emote.shortcode }),
    label: t('emotes.aliasesPrompt'),
    value: (emote.aliases ?? []).filter((a) => a !== emote.name).join(', '),
    allowEmpty: true,
    confirm: t('common.save'),
  })
  if (next !== null)
    await run(
      () =>
        apiClient().updateEmote(emote.id, {
          aliases: next
            .split(',')
            .map((a) => a.trim())
            .filter(Boolean),
        }),
      'emotes.aliasesSaved',
    )
}
const toggle = (emote: InstanceEmote) =>
  run(
    () => apiClient().updateEmote(emote.id, { enabled: !emote.enabled }),
    emote.enabled ? 'emotes.disabled' : 'emotes.enabled',
  )
async function remove(emote: InstanceEmote) {
  if (
    await confirm({
      title: t('emotes.remove.title', { shortcode: emote.shortcode }),
      body: t('emotes.remove.body', { shortcode: emote.shortcode }),
      confirm: t('common.remove'),
      danger: true,
    })
  )
    await run(() => apiClient().deleteEmote(emote.id), 'emotes.removed')
}
const { animate } = storeToRefs(useAppearanceStore())
const displayName = computed(
  () => chat.self?.displayName || chat.self?.nick || auth.account?.username || '',
)
</script>
<template>
  <div
    class="appearance-settings emote-settings"
    @setting-reveal="openUpload"
  >
    <SettingsDescription>
      <i18n-t keypath="emotes.intro">
        <template #code><code>:name:</code></template>
      </i18n-t>
    </SettingsDescription>
    <UiAlert
      v-if="error && panel === 'closed'"
      as="p"
      variant="destructive"
      presentation="inline"
      role="alert"
      >{{ error }}</UiAlert
    >
    <FieldHint
      v-if="status && panel === 'closed'"
      role="status"
      >{{ $t(status) }}</FieldHint
    >
    <UiRadioGroup
      :model-value="tab"
      class="emote-tabs"
      :aria-label="$t('emotes.browse')"
      @update:model-value="changeTab"
    >
      <UiRadioGroupItem
        value="server"
        variant="segment"
        class="emote-tab"
      >
        {{ $t('emotes.serverTab') }} <span class="emote-count">{{ n(items.length) }}</span>
      </UiRadioGroupItem>
      <UiRadioGroupItem
        value="discover"
        variant="segment"
        class="emote-tab"
        >{{ $t('emotes.discover') }}</UiRadioGroupItem
      >
    </UiRadioGroup>
    <div class="emote-controls">
      <div class="emote-toolbar">
        <div class="emote-search">
          <AppIcon
            name="search"
            :size="16"
            aria-hidden="true"
          />
          <UiInput
            v-if="tab === 'server'"
            id="emote-search"
            v-model="search"
            :aria-label="$t('emotes.searchServer')"
            :placeholder="$t('emotes.searchServer')"
          />
          <UiInput
            v-else
            id="discover-search"
            v-model="discoverQuery"
            maxlength="100"
            :aria-label="$t('emotes.searchDiscover')"
            :placeholder="$t('emotes.searchDiscover')"
          />
        </div>
        <UiButton
          v-if="canAdd"
          data-setting="add-emote"
          size="sm"
          class="emote-upload"
          @click="openUpload"
        >
          <AppIcon
            name="upload"
            :size="16"
            aria-hidden="true"
          />{{ $t('emotes.uploadImage') }}
        </UiButton>
      </div>
      <div
        v-if="tab === 'discover' && discoverPage"
        class="emote-filters"
      >
        <label
          v-if="(discoverPage.filters.sources?.length ?? 0) > 1"
          class="emote-source-filter"
        >
          <span class="sr-only">{{ $t('emotes.sourceFilter') }}</span>
          <select
            v-model="discoverSource"
            class="control-input"
            :aria-label="$t('emotes.sourceFilter')"
          >
            <option value="">{{ $t('emotes.allSources') }}</option>
            <option
              v-for="sourceName in discoverPage.filters.sources ?? []"
              :key="sourceName"
              :value="sourceName"
            >
              {{ sourceName }}
            </option>
          </select>
        </label>
        <span
          v-else-if="discoverPage.filters.sources?.[0]"
          class="emote-provider-pill"
          >{{ discoverPage.filters.sources[0] }}</span
        >
        <UiRadioGroup
          v-model="discoverAnimation"
          class="emote-animation"
          :aria-label="$t('emotes.animation')"
        >
          <UiRadioGroupItem
            v-for="option in discoverPage.filters.animations ?? []"
            :key="option"
            :value="option"
            variant="segment"
            class="emote-animation-option"
            >{{ $t(`emotes.animationOptions.${option}`) }}</UiRadioGroupItem
          >
        </UiRadioGroup>
      </div>
    </div>
    <div class="emote-workspace">
      <div class="emote-content">
        <section
          v-if="tab === 'server'"
          data-setting="server-emotes"
          class="setting-group grid gap-3.5"
        >
          <div
            v-if="visibleItems.length"
            class="emote-list-heading"
            aria-hidden="true"
          >
            <span>{{ $t('emotes.image') }}</span
            ><span>{{ $t('emotes.shortcode') }}</span
            ><span>{{ $t('emotes.sourceFilter') }}</span
            ><span />
          </div>
          <div
            v-if="catalogError && !catalog"
            class="emote-state"
            role="alert"
          >
            <AppIcon
              name="alert"
              :size="22"
              aria-hidden="true"
            /><strong>{{ $t('emotes.unavailable') }}</strong
            ><span>{{ $t('emotes.tryLater') }}</span>
          </div>
          <div
            v-else-if="!catalog"
            class="emote-state"
            role="status"
          >
            {{ $t('emotes.loadingServer') }}
          </div>
          <div
            v-else-if="!items.length"
            class="emote-state"
          >
            <AppIcon
              name="smile"
              :size="22"
              aria-hidden="true"
            /><strong>{{ $t('emotes.none') }}</strong
            ><span>{{ $t(canAdd ? 'emotes.emptyAdd' : 'emotes.emptyBrowse') }}</span>
          </div>
          <div
            v-else-if="!visibleItems.length"
            class="emote-state"
          >
            <AppIcon
              name="search"
              :size="22"
              aria-hidden="true"
            /><strong>{{ $t('emotes.noMatches') }}</strong
            ><span>{{ $t('emotes.adjustSearch') }}</span>
          </div>
          <ul class="emote-list list-none m-0 p-0">
            <li
              v-for="emote in visibleItems"
              :key="emote.id"
              class="emote-row"
              :class="{ disabled: !emote.enabled }"
            >
              <NuxtImg
                class="emote-thumb"
                :src="animate ? emote.url : emote.staticUrl"
                :alt="`:${emote.shortcode}:`"
                width="32"
                height="32"
              />
              <span class="emote-name">
                <strong
                  >:{{ emote.shortcode }}:<span v-if="!emote.enabled">
                    {{ $t('emotes.disabledTag') }}</span
                  ></strong
                >
                <small v-if="emote.aliases?.length">{{
                  $t('emotes.also', { aliases: emote.aliases.join(', ') })
                }}</small>
              </span>
              <span class="emote-source">
                <template v-if="emote.source !== 'upload'"
                  ><a
                    :href="emote.sourceUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                    >{{ providerName(emote.source) }}</a
                  ></template
                >
                <template v-else>{{ $t('emotes.uploaded') }}</template>
                <small v-if="emote.creator">{{
                  $t('emotes.by', { creator: emote.creator })
                }}</small>
              </span>
              <details
                v-if="mayManage(emote)"
                class="emote-actions"
              >
                <summary :aria-label="$t('emotes.manageAria', { shortcode: emote.shortcode })">
                  <AppIcon
                    name="more"
                    :size="18"
                    aria-hidden="true"
                  />
                </summary>
                <div class="emote-action-menu">
                  <UiButton
                    variant="secondary"
                    size="sm"
                    :disabled="busy"
                    @click="rename(emote)"
                    >{{ $t('emotes.rename.label') }}</UiButton
                  >
                  <UiButton
                    variant="secondary"
                    size="sm"
                    :disabled="busy"
                    @click="editAliases(emote)"
                    >{{ $t('emotes.aliasesButton') }}</UiButton
                  >
                  <AppSwitch
                    :model-value="emote.enabled"
                    :aria-label="$t('emotes.enableAria', { shortcode: emote.shortcode })"
                    :disabled="busy"
                    @update:model-value="toggle(emote)"
                  />
                  <UiButton
                    variant="destructive"
                    size="sm"
                    :disabled="busy"
                    @click="remove(emote)"
                    >{{ $t('common.remove') }}</UiButton
                  >
                </div>
              </details>
            </li>
          </ul>
        </section>
        <section
          v-else
          class="setting-group grid gap-3.5"
        >
          <FieldHint v-if="discoverPage?.stale">{{ $t('emotes.stale') }}</FieldHint>
          <div
            v-if="discoverError"
            class="emote-state emote-error"
            role="alert"
          >
            <AppIcon
              name="alert"
              :size="22"
              aria-hidden="true"
            />
            <strong>{{ $t('emotes.discoveryUnavailable') }}</strong>
            <span>{{ discoverError }}</span>
            <UiButton
              variant="secondary"
              size="sm"
              @click="discovery.restart"
              >{{ $t('emotes.retry') }}</UiButton
            >
          </div>
          <div
            v-if="discoverLoading"
            class="emote-grid emote-loading"
            role="status"
            :aria-label="$t('emotes.loading')"
          >
            <span
              v-for="index in 6"
              :key="index"
              class="emote-skeleton"
              aria-hidden="true"
            />
          </div>
          <div
            v-else-if="!discoverError && discoverPage && !discoverPage.items?.length"
            class="emote-state"
            role="status"
          >
            <AppIcon
              name="search"
              :size="22"
              aria-hidden="true"
            />
            <strong>{{
              $t(
                discoverPage.nextCursor || cursors.length > 1
                  ? 'emotes.noPageMatches'
                  : 'emotes.noDiscoverMatches',
              )
            }}</strong>
            <span>{{
              $t(discoverPage.nextCursor ? 'emotes.morePageMatches' : 'emotes.adjustFilters')
            }}</span>
          </div>
          <div
            v-if="!discoverLoading && !discoverError && discoverPage?.items?.length"
            class="emote-grid"
          >
            <button
              v-for="item in discoverPage.items"
              :key="`${item.provider}:${item.providerId}`"
              type="button"
              class="emote-tile"
              :aria-pressed="selected?.providerId === item.providerId && panel === 'discover'"
              @click="selectDiscovery(item, $event)"
            >
              <EmoteStillPreview
                :src="previewSource(item)"
                :label="item.name"
              />
              <span class="emote-tile-name">:{{ item.name }}:</span>
              <small
                v-if="item.animated"
                class="emote-tile-animation"
                >{{ $t('emotes.animated') }}</small
              >
              <small
                v-if="isInstalled(item)"
                class="emote-installed"
                >{{ $t('emotes.installed') }}</small
              >
            </button>
          </div>
          <div
            v-if="!discoverError && discoverPage && (cursors.length > 1 || discoverPage.nextCursor)"
            class="emote-pagination"
          >
            <UiButton
              variant="secondary"
              size="sm"
              :disabled="cursors.length < 2 || discoverLoading"
              @click="discovery.previous"
              >{{ $t('emotes.previous') }}</UiButton
            >
            <UiButton
              variant="secondary"
              size="sm"
              :disabled="!discoverPage.nextCursor || discoverLoading"
              @click="discovery.next"
              >{{ $t('emotes.next') }}</UiButton
            >
          </div>
        </section>
      </div>
      <section
        v-if="panel !== 'closed'"
        class="emote-detail setting-group grid gap-2.5"
      >
        <div class="emote-detail-heading">
          <h3
            ref="detailHeading"
            tabindex="-1"
          >
            {{
              mode === 'discover'
                ? selected?.name
                : mode === 'upload'
                  ? $t('emotes.uploadImage')
                  : $t('emotes.importProvider')
            }}
          </h3>
          <UiButton
            variant="secondary"
            size="sm"
            @click="closeDetail"
            >{{ $t('common.close') }}</UiButton
          >
        </div>
        <AppSegmented
          v-if="mode !== 'discover'"
          :model-value="mode"
          :label="$t('emotes.howToAdd')"
          :options="[
            { value: 'upload', label: $t('emotes.uploadImage') },
            { value: 'import', label: $t('emotes.importProvider') },
          ]"
          @update:model-value="panel = $event as 'upload' | 'import'"
        />
        <form
          class="emote-form grid gap-2.5"
          @submit.prevent="add"
        >
          <template v-if="mode === 'upload'">
            <AppField
              id="emote-file"
              :label="$t('emotes.image')"
              :help="
                $t('emotes.imageHelp', { size: $n(Math.round((catalog?.maxBytes ?? 0) / 1024)) })
              "
            >
              <template #default="control">
                <div
                  class="emote-drop"
                  @dragover.prevent
                  @drop.prevent="dropFile"
                >
                  <img
                    v-if="animate ? previewUrl : stillUrl"
                    :src="animate ? previewUrl : stillUrl"
                    alt=""
                    class="emote-preview-image"
                  />
                  <input
                    v-bind="control"
                    :key="fileInput"
                    class="emote-file-input"
                    type="file"
                    :accept="(catalog?.types ?? []).join(',')"
                    @change="chooseFile"
                  />
                  <span
                    v-if="file"
                    class="emote-file-name"
                    >{{ file.name }}</span
                  >
                  <label
                    for="emote-file"
                    class="emote-file-trigger"
                    >{{ $t(file ? 'emotes.chooseAnother' : 'emotes.chooseFile') }}</label
                  >
                </div>
              </template>
            </AppField>
          </template>
          <div
            v-else-if="mode === 'discover' && selected"
            class="emote-provider-preview"
          >
            <div class="emote-preview-art">
              <EmoteStillPreview
                :src="previewSource(selected)"
                :label="selected.name"
                :playing="animate && selected.animated"
              /><span
                v-if="selected.animated"
                class="emote-animation-badge"
                >{{ $t('emotes.animated') }}</span
              >
            </div>
            <p>
              {{ $t('emotes.from') }}
              <a
                :href="selected.sourceUrl"
                target="_blank"
                rel="noopener noreferrer"
                >{{ selected.provider }}</a
              ><template v-if="selected.creator"
                >{{ ' ' }}{{ $t('emotes.by', { creator: selected.creator }) }}</template
              ><template v-if="selected.provider === 'emoji.gg'"
                >{{ ' · '
                }}<a
                  href="https://emoji.gg/licenses"
                  target="_blank"
                  rel="noopener noreferrer"
                  >{{ $t('emotes.usageTerms') }}</a
                ></template
              >
            </p>
          </div>
          <AppInput
            v-else
            id="emote-source"
            v-model="source"
            :label="$t('emotes.source')"
            :help="$t('emotes.sourceHelp')"
            :placeholder="sourcePlaceholder"
          />
          <AppInput
            id="emote-shortcode"
            v-model="shortcode"
            :label="$t('emotes.shortcode')"
            maxlength="34"
            :placeholder="mode === 'import' ? $t('emotes.shortcodeDefault') : 'kekw'"
          />
          <AppInput
            id="emote-aliases"
            v-model="aliases"
            maxlength="300"
          >
            <template #label
              >{{ $t('emotes.aliasesLabel') }}
              <small>{{ $t('emotes.aliasesOptional') }}</small></template
            >
          </AppInput>
          <div
            v-if="(mode === 'upload' && file) || (mode === 'discover' && selected)"
            class="emote-chat-preview"
          >
            <strong>{{ $t('emotes.chatPreview') }}</strong>
            <div class="emote-chat-message">
              <UserAvatar :member="chat.self || { displayName }" />
              <div class="min-w-0">
                <strong :style="chat.self ? nameStyle(chat.self) : undefined">{{
                  displayName
                }}</strong>
                <div class="message-text font-chat leading-[1.5] text-body">
                  {{ $t('emotes.previewMessage') }}
                  <img
                    v-if="mode === 'upload' && (animate ? previewUrl : stillUrl)"
                    class="inline-custom-emoji"
                    :src="animate ? previewUrl : stillUrl"
                    :alt="`:${shortcode}:`"
                  /><template v-else-if="mode === 'discover' && selected"
                    ><EmoteStillPreview
                      class="inline-custom-emoji"
                      :src="previewSource(selected)"
                      :label="shortcode"
                      :playing="animate && selected.animated"
                  /></template>
                </div>
              </div>
            </div>
          </div>
          <UiAlert
            v-if="error"
            as="p"
            variant="destructive"
            presentation="inline"
            class="form-error mt-3"
          >
            {{ error }}
          </UiAlert>
          <FieldHint
            v-if="status"
            role="status"
          >
            {{ $t(status) }}
          </FieldHint>
          <div class="form-actions flex justify-end items-center gap-2 mt-2">
            <UiButton
              size="sm"
              :disabled="
                busy ||
                !canAdd ||
                (mode === 'upload'
                  ? !file || !shortcode
                  : mode === 'discover'
                    ? !selected || isInstalled(selected) || !shortcode
                    : !source)
              "
            >
              {{
                mode === 'discover' && selected && isInstalled(selected)
                  ? $t('emotes.installed')
                  : $t('emotes.addToServer')
              }}
            </UiButton>
          </div>
        </form>
      </section>
    </div>
  </div>
</template>

<style scoped>
.emote-settings {
  display: grid;
  gap: 1rem;
}
.emote-tabs {
  display: flex;
  gap: 1rem;
  border-bottom: 1px solid var(--border);
}
.emote-tab {
  position: relative;
  min-height: 2.5rem;
  padding: 0 0.5rem;
  border-radius: 0;
  font-size: var(--ui-text);
}
.emote-tab[data-state='checked'] {
  background: transparent;
  box-shadow: none;
  font-weight: 650;
}
.emote-tab[data-state='checked']::after {
  content: '';
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: var(--accent);
}
.emote-tabs :deep([role='radio']) {
  position: relative;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}
.emote-tabs :deep([role='radio'][data-state='checked']) {
  background: transparent;
  box-shadow: none;
  font-weight: 650;
}
.emote-tabs :deep([role='radio'][data-state='checked']::after) {
  content: '';
  position: absolute;
  right: 0;
  bottom: -1px;
  left: 0;
  height: 2px;
  background: var(--accent);
}
.emote-count {
  min-width: 1.4rem;
  padding: 0.1rem 0.35rem;
  border-radius: 1rem;
  background: var(--raised);
  color: var(--muted);
  font-size: var(--ui-caption);
  text-align: center;
}
.emote-controls {
  display: grid;
  gap: 0.75rem;
}
.emote-search {
  position: relative;
  flex: 1 1 auto;
  min-width: 0;
}
.emote-search > svg {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--muted);
  pointer-events: none;
}
.emote-search :deep(input) {
  width: 100%;
  padding-left: 2.25rem;
}
.emote-upload {
  flex: none;
  gap: 0.5rem;
}
.emote-filters {
  align-items: center;
  flex-wrap: nowrap;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border);
}
.emote-filters .emote-source-filter {
  display: block;
  flex: 0 1 8rem;
  min-width: 6rem;
}
.emote-filters select {
  min-height: 2rem;
  padding-block: 0.25rem;
}
.emote-provider-pill {
  padding: 0.35rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-animation {
  display: flex;
  margin-left: auto;
  gap: 0.125rem;
  padding: 0.125rem;
  border-radius: var(--radius);
  background: var(--panel);
}
.emote-animation-option {
  min-height: 2rem;
  padding-inline: 0.55rem;
}
.emote-workspace {
  display: flex;
  gap: 1.5rem;
  min-width: 0;
  align-items: start;
}
.emote-content {
  flex: 1 1 0;
  min-width: 0;
}
.emote-detail {
  flex: 0 0 min(35%, 19rem);
  min-width: 15rem;
  border-left: 1px solid var(--border);
  padding-left: 1.5rem;
}
.emote-detail-heading {
  align-items: center;
}
.emote-detail-heading h3 {
  min-width: 0;
  overflow-wrap: anywhere;
  padding: 0;
  border: 0;
  color: var(--text);
  font-size: var(--ui-heading);
  font-weight: 650;
  text-transform: none;
  letter-spacing: normal;
}
.emote-list-heading,
.emote-row {
  display: grid;
  grid-template-columns: 3rem minmax(5rem, 1fr) minmax(5rem, 1fr) 2.25rem;
  align-items: center;
  gap: 0.75rem;
}
.emote-list-heading {
  padding: 0.25rem 0.25rem 0.5rem;
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-row {
  min-height: 4rem;
  padding: 0.5rem 0.25rem;
  border-top: 1px solid var(--border);
}
.emote-row:last-child {
  border-bottom: 1px solid var(--border);
}
.emote-row.disabled .emote-thumb {
  opacity: 0.45;
}
.emote-thumb {
  width: 2.75rem;
  height: 2.75rem;
  object-fit: contain;
}
.emote-name,
.emote-source {
  display: grid;
  gap: 0.125rem;
  min-width: 0;
  overflow-wrap: anywhere;
}
.emote-name strong {
  color: var(--text);
}
.emote-name small,
.emote-source small {
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-source a {
  color: inherit;
  text-decoration: underline;
  text-underline-offset: 0.15em;
}
.emote-actions summary {
  display: grid;
  place-items: center;
  width: 2.25rem;
  min-height: 2.25rem;
  padding: 0;
  border: 0;
  color: var(--muted);
}
.emote-actions summary:hover {
  color: var(--text);
  background: var(--hover);
}
.emote-state {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
  min-height: 11rem;
  align-content: center;
  padding: 1.5rem;
  color: var(--muted);
  text-align: center;
}
.emote-state strong {
  color: var(--text);
}
.emote-skeleton {
  height: 7.5rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--panel);
}
.emote-toolbar,
.emote-detail-heading {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 0.75rem;
}
.emote-toolbar > :first-child {
  flex: 1;
  min-width: 0;
}
.emote-drop {
  display: grid;
  gap: 0.3rem;
  justify-items: center;
  padding: 0.75rem;
  border: 1px dashed var(--border);
  border-radius: 0.5rem;
}
.emote-drop:focus-within {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.emote-file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
.emote-file-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
.emote-file-trigger {
  color: var(--accent);
  cursor: pointer;
  font-weight: 600;
  font-size: var(--ui-caption);
}
.emote-preview-image {
  width: 4.5rem;
  height: 4.5rem;
  object-fit: contain;
}
.emote-chat-preview {
  display: grid;
  gap: 0.5rem;
}
.emote-chat-message {
  display: flex;
  gap: 0.75rem;
  align-items: start;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  padding: 0.75rem;
}
.emote-actions {
  position: relative;
}
.emote-actions summary {
  cursor: pointer;
  border-radius: 0.4rem;
  list-style: none;
}
.emote-actions summary::-webkit-details-marker {
  display: none;
}
.emote-action-menu {
  position: absolute;
  z-index: 2;
  right: 0;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  width: max-content;
  max-width: min(90vw, 24rem);
  flex-wrap: wrap;
  padding: 0.5rem;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--card);
}
.emote-filters {
  display: flex;
  flex-wrap: nowrap;
  gap: 0.75rem;
}
.emote-filters label {
  display: grid;
  gap: 0.25rem;
  flex: 1 1 8rem;
  color: var(--muted);
  font-size: 0.8rem;
}
.emote-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(6rem, 1fr));
  gap: 0.5rem;
}
.emote-tile {
  position: relative;
  display: grid;
  align-content: start;
  justify-items: center;
  gap: 0.3rem;
  min-width: 0;
  min-height: 7.5rem;
  padding: 0.75rem 0.4rem 0.5rem;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--card);
}
.emote-tile:hover,
.emote-tile[aria-pressed='true'] {
  border-color: var(--accent);
}
.emote-tile .emote-still {
  width: 4.5rem;
  height: 4.5rem;
  object-fit: contain;
}
.emote-tile-name {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.emote-tile small {
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-tile .emote-tile-animation {
  position: absolute;
  top: 0.35rem;
  right: 0.35rem;
  padding: 0.05rem 0.3rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-small);
  background: var(--raised);
  font-size: 0.65rem;
}
.emote-tile .emote-installed {
  padding: 0.05rem 0.4rem;
  border-radius: var(--radius-small);
  background: var(--selection);
  color: var(--text);
}
.emote-pagination {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
}
.emote-provider-preview {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
}
.emote-preview-art {
  display: grid;
  place-items: center;
  position: relative;
  width: 100%;
  min-height: 8rem;
  padding: 0.75rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--panel);
}
.emote-preview-art .emote-still {
  width: 7rem;
  height: 7rem;
  object-fit: contain;
}
.emote-animation-badge {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  padding: 0.1rem 0.4rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-small);
  background: var(--raised);
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-provider-preview p {
  justify-self: start;
  color: var(--muted);
  font-size: var(--ui-caption);
}
.emote-chat-message {
  background: var(--panel);
}
.emote-form .form-actions :deep(button) {
  width: 100%;
}
@media (max-width: 700px) {
  .emote-settings:has(.emote-detail) .emote-controls {
    display: none;
  }
  .emote-workspace {
    display: block;
  }
  .emote-workspace:has(.emote-detail) .emote-content {
    display: none;
  }
  .emote-detail {
    min-width: 0;
    border-left: 0;
    padding-left: 0;
  }
  .emote-filters {
    flex-wrap: wrap;
  }
  .emote-animation {
    margin-left: auto;
  }
  .emote-list-heading,
  .emote-row {
    grid-template-columns: 2.5rem minmax(4rem, 1fr) minmax(4rem, 0.8fr) 2.25rem;
    gap: 0.5rem;
  }
  .emote-thumb {
    width: 2.25rem;
    height: 2.25rem;
  }
}
</style>
