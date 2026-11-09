<script setup lang="ts">
/**
 * Settings → Emotes: the server's shared catalog. Everyone sees it; accounts listed in
 * IRCORD_EMOTE_MANAGERS can upload, import from 7TV or FrankerFaceZ, rename, disable and remove.
 */
import type { InstanceEmote } from '../../shared/types/media'
import { parseProviderEmote, providerName } from '../../shared/utils/media'

const media = useMediaStore()
const queryCache = useQueryCache()
const { ask, confirm } = usePrompt()
const { t, n } = useI18n()
media.needCustom()
const catalog = computed(() => media.instanceQuery.data)
const catalogError = computed(() => media.instanceQuery.error)
const canManage = computed(() => !!catalog.value?.canManage)
const items = computed(() => catalog.value?.items ?? [])

const mode = ref<'upload' | 'import'>('upload')
const file = ref<File>()
/** Bumped to render a fresh, empty file input after an upload. */
const fileInput = ref(0)
const shortcode = ref('')
const aliases = ref('')
const source = ref('')
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
    await refresh()
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    busy.value = false
  }
}

function chooseFile(event: Event) {
  file.value = (event.target as HTMLInputElement).files?.[0]
  if (file.value && !shortcode.value)
    shortcode.value = file.value.name
      .replace(/\.[^.]+$/, '')
      .replace(/[^\w]+/g, '_')
      .slice(0, 32)
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

function add() {
  if (mode.value === 'upload') {
    const image = file.value
    if (!image) return
    return run(async () => {
      await apiClient().uploadEmote({
        file: image,
        still: await stillFrame(image),
        shortcode: shortcode.value,
        aliases: aliases.value || undefined,
      })
      file.value = undefined
      fileInput.value++
      shortcode.value = aliases.value = ''
    }, 'emotes.added')
  }
  const parsed = parseProviderEmote(source.value)
  if (!parsed) {
    error.value = t('emotes.pasteLink')
    return
  }
  return run(async () => {
    await apiClient().importEmote({
      ...parsed,
      shortcode: shortcode.value || undefined,
      aliases: aliases.value
        .split(',')
        .map((a) => a.trim())
        .filter(Boolean),
    })
    source.value = shortcode.value = aliases.value = ''
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
</script>
<template>
  <div class="appearance-settings grid gap-6 emote-settings">
    <SettingsDescription>
      <i18n-t keypath="emotes.intro">
        <template #code><code>:name:</code></template>
      </i18n-t>
    </SettingsDescription>
    <section
      v-if="canManage"
      data-setting="add-emote"
      class="setting-group grid gap-3.5"
    >
      <h3>{{ $t('emotes.add') }}</h3>
      <AppSegmented
        v-model="mode"
        :label="$t('emotes.howToAdd')"
        :options="[
          { value: 'upload', label: $t('emotes.uploadImage') },
          { value: 'import', label: $t('emotes.importProvider') },
        ]"
      />
      <form
        class="emote-form grid gap-4 mt-3"
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
              <input
                v-bind="control"
                :key="fileInput"
                class="control-input"
                type="file"
                :accept="(catalog?.types ?? []).join(',')"
                @change="chooseFile"
              />
            </template>
          </AppField>
        </template>
        <AppInput
          v-else
          id="emote-source"
          v-model="source"
          :label="$t('emotes.source')"
          :help="$t('emotes.sourceHelp')"
          placeholder="https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD"
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
          v-else-if="status"
          role="status"
        >
          {{ $t(status) }}
        </FieldHint>
        <div class="form-actions flex justify-end items-center gap-2 mt-5">
          <UiButton
            size="sm"
            :disabled="busy || (mode === 'upload' ? !file || !shortcode : !source)"
          >
            {{ mode === 'upload' ? $t('emotes.upload') : $t('emotes.import') }}
          </UiButton>
        </div>
      </form>
    </section>
    <section
      data-setting="server-emotes"
      class="setting-group grid gap-3.5"
    >
      <h3>{{ $t('emotes.serverEmotes', { count: n(items.length) }) }}</h3>
      <FieldHint v-if="catalogError && !catalog">
        {{ $t('emotes.unavailable') }}
      </FieldHint>
      <FieldHint v-else-if="!items.length">{{ $t('emotes.none') }}</FieldHint>
      <ul class="emote-list list-none m-0 p-0">
        <li
          v-for="emote in items"
          :key="emote.id"
          class="setting-row flex items-center justify-between emote-row gap-3 [&amp;.disabled_.emote-thumb]:opacity-45 max-[560px]:flex-wrap"
          :class="{ disabled: !emote.enabled }"
        >
          <NuxtImg
            class="emote-thumb size-8 object-contain flex-none"
            :src="animate ? emote.url : emote.staticUrl"
            :alt="`:${emote.shortcode}:`"
            width="32"
            height="32"
          />
          <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
            ><strong
              >:{{ emote.shortcode }}:<span v-if="!emote.enabled">
                {{ $t('emotes.disabledTag') }}</span
              ></strong
            ><small
              ><template v-if="emote.aliases?.length"
                >{{ $t('emotes.also', { aliases: emote.aliases.join(', ') }) }} </template
              ><template v-if="emote.source !== 'upload'"
                >{{ $t('emotes.from') }}
                <a
                  :href="emote.sourceUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  >{{ providerName(emote.source) }}</a
                ></template
              ><template v-if="emote.creator">
                {{ $t('emotes.by', { creator: emote.creator }) }}</template
              ></small
            ></span
          >
          <span
            v-if="canManage"
            class="emote-actions flex items-center gap-1.5 flex-wrap justify-end max-[560px]:w-full"
          >
            <UiButton
              variant="secondary"
              size="sm"
              :disabled="busy"
              @click="rename(emote)"
            >
              {{ $t('emotes.rename.label') }}
            </UiButton>
            <UiButton
              variant="secondary"
              size="sm"
              :disabled="busy"
              @click="editAliases(emote)"
            >
              {{ $t('emotes.aliasesButton') }}
            </UiButton>
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
            >
              {{ $t('common.remove') }}
            </UiButton>
          </span>
        </li>
      </ul>
    </section>
  </div>
</template>
