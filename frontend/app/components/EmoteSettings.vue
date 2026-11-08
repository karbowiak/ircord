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
media.needCustom()
const catalog = computed(() => media.instanceQuery.data)
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
async function run(action: () => Promise<unknown>, done: string) {
  busy.value = true
  error.value = status.value = ''
  try {
    await action()
    status.value = done
    await refresh()
  } catch (e) {
    error.value = errorText(e)
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
    }, 'Emote added.')
  }
  const parsed = parseProviderEmote(source.value)
  if (!parsed) {
    error.value = 'Paste a 7TV or FrankerFaceZ emote link.'
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
  }, 'Emote imported.')
}

async function rename(emote: InstanceEmote) {
  const next = await ask({
    title: `Rename :${emote.shortcode}:`,
    body: 'Old messages keep showing the emote under its former name.',
    label: 'Shortcode',
    value: emote.shortcode,
    confirm: 'Rename',
  })
  if (next && next !== emote.shortcode)
    await run(() => apiClient().updateEmote(emote.id, { shortcode: next }), 'Emote renamed.')
}
async function editAliases(emote: InstanceEmote) {
  const next = await ask({
    title: `Aliases for :${emote.shortcode}:`,
    label: 'Comma-separated aliases',
    value: (emote.aliases ?? []).filter((a) => a !== emote.name).join(', '),
    allowEmpty: true,
    confirm: 'Save',
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
      'Aliases saved.',
    )
}
const toggle = (emote: InstanceEmote) =>
  run(
    () => apiClient().updateEmote(emote.id, { enabled: !emote.enabled }),
    emote.enabled ? 'Emote disabled.' : 'Emote enabled.',
  )
async function remove(emote: InstanceEmote) {
  if (
    await confirm({
      title: `Remove :${emote.shortcode}:?`,
      body:
        'Everyone on this server loses it. Messages that used it show :' +
        emote.shortcode +
        ': as text.',
      confirm: 'Remove',
      danger: true,
    })
  )
    await run(() => apiClient().deleteEmote(emote.id), 'Emote removed.')
}
const appearance = useAppearanceStore()
</script>
<template>
  <div class="appearance-settings grid gap-6 emote-settings">
    <p class="settings-description text-[13px] text-muted-foreground leading-[1.55]">
      Custom emotes shared by everyone on this IRCord server. Type <code>:name:</code> or pick them
      under Server emoji; other IRC clients see the name as text.
    </p>
    <section
      v-if="canManage"
      data-setting="add-emote"
      class="setting-group grid gap-3.5"
    >
      <h3>Add an emote</h3>
      <div
        class="segmented"
        role="group"
        aria-label="How to add"
      >
        <button
          :class="{ active: mode === 'upload' }"
          :aria-pressed="mode === 'upload'"
          @click="mode = 'upload'"
        >
          Upload image
        </button>
        <button
          :class="{ active: mode === 'import' }"
          :aria-pressed="mode === 'import'"
          @click="mode = 'import'"
        >
          Import from 7TV or FFZ
        </button>
      </div>
      <form
        class="emote-form grid gap-1.5 mt-3"
        @submit.prevent="add"
      >
        <template v-if="mode === 'upload'">
          <label
            class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
            for="emote-file"
            >Image</label
          >
          <input
            id="emote-file"
            :key="fileInput"
            class="control-input"
            type="file"
            :accept="(catalog?.types ?? []).join(',')"
            aria-describedby="emote-file-hint"
            @change="chooseFile"
          />
          <p
            id="emote-file-hint"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            PNG, GIF, WebP or JPEG up to {{ Math.round((catalog?.maxBytes ?? 0) / 1024) }} KiB.
          </p>
        </template>
        <template v-else>
          <label
            class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
            for="emote-source"
            >Emote link or ID</label
          >
          <input
            id="emote-source"
            v-model="source"
            class="control-input"
            placeholder="https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD"
            aria-describedby="emote-source-hint"
          />
          <p
            id="emote-source-hint"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            The image is copied to this server; the link and creator are kept for credit.
          </p>
        </template>
        <label
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="emote-shortcode"
          >Shortcode</label
        >
        <input
          id="emote-shortcode"
          v-model="shortcode"
          class="control-input"
          maxlength="34"
          :placeholder="mode === 'import' ? 'Defaults to the emote’s name' : 'kekw'"
        />
        <label
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="emote-aliases"
          >Aliases <small>(optional, comma-separated)</small></label
        >
        <input
          id="emote-aliases"
          v-model="aliases"
          class="control-input"
          maxlength="300"
        />
        <p
          v-if="error"
          class="form-error text-destructive text-[13px] mt-3"
          role="alert"
        >
          {{ error }}
        </p>
        <p
          v-else-if="status"
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          role="status"
        >
          {{ status }}
        </p>
        <div class="form-actions flex justify-end items-center gap-2 mt-5">
          <button
            class="button primary small"
            :disabled="busy || (mode === 'upload' ? !file || !shortcode : !source)"
          >
            {{ mode === 'upload' ? 'Upload' : 'Import' }}
          </button>
        </div>
      </form>
    </section>
    <section
      data-setting="server-emotes"
      class="setting-group grid gap-3.5"
    >
      <h3>Server emotes ({{ items.length }})</h3>
      <p
        v-if="media.instanceQuery.error && !catalog"
        class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
      >
        Server emotes are unavailable right now.
      </p>
      <p
        v-else-if="!items.length"
        class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
      >
        No server emotes yet.
      </p>
      <ul class="emote-list list-none m-0 p-0">
        <li
          v-for="emote in items"
          :key="emote.id"
          class="setting-row flex items-center justify-between emote-row gap-3 [&amp;.disabled_.emote-thumb]:opacity-45 max-[560px]:flex-wrap"
          :class="{ disabled: !emote.enabled }"
        >
          <img
            class="emote-thumb size-8 object-contain flex-none"
            :src="appearance.animate ? emote.url : emote.staticUrl"
            :alt="`:${emote.shortcode}:`"
            width="32"
            height="32"
          />
          <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
            ><strong>:{{ emote.shortcode }}:<span v-if="!emote.enabled"> (disabled)</span></strong
            ><small
              ><template v-if="emote.aliases?.length"
                >Also {{ emote.aliases.join(', ') }}. </template
              ><template v-if="emote.source !== 'upload'"
                >From
                <a
                  :href="emote.sourceUrl"
                  target="_blank"
                  rel="noopener noreferrer"
                  >{{ providerName(emote.source) }}</a
                ></template
              ><template v-if="emote.creator"> by {{ emote.creator }}</template></small
            ></span
          >
          <span
            v-if="canManage"
            class="emote-actions flex items-center gap-1.5 flex-wrap justify-end max-[560px]:w-full"
          >
            <button
              class="button secondary small"
              :disabled="busy"
              @click="rename(emote)"
            >
              Rename
            </button>
            <button
              class="button secondary small"
              :disabled="busy"
              @click="editAliases(emote)"
            >
              Aliases
            </button>
            <button
              role="switch"
              class="toggle"
              :class="{ on: emote.enabled }"
              :aria-checked="emote.enabled"
              :aria-label="`Enable :${emote.shortcode}:`"
              :disabled="busy"
              @click="toggle(emote)"
            >
              <span />
            </button>
            <button
              class="button small danger"
              :disabled="busy"
              @click="remove(emote)"
            >
              Remove
            </button>
          </span>
        </li>
      </ul>
    </section>
  </div>
</template>
