<!-- Not Vapor: it renders inside ThemeLayer's shadow root (see there). -->
<script setup lang="ts">
/**
 * The theme studio (#88): edits the draft theme while the live app beside it shows the result,
 * in this tab only. Nothing is written to the account, other tabs or the pre-paint copy until
 * Save. Native controls only: the app's components and stylesheets don't reach the shadow root.
 */
import {
  schemeFamilies,
  schemeFor,
  schemeTokens,
  schemes,
  type SchemeMode,
} from '../../../../shared/schemes'
import {
  backgroundFloors,
  backgroundOptions,
  backgroundRanges,
  defaultBackground,
  instanceFilePath,
  measureTones,
  paletteKeys,
  seedKeys,
  seedsOf,
  themeLimits,
  themeScheme,
  type ThemeBackground,
} from '../../../../shared/themes'
import { cssRemovalSummary } from './css-report'

const themes = useThemesStore()
const appearance = useAppearanceStore()
const chat = useChatStore()
const { t } = useI18n()
const $t = t

const draft = computed(() => themes.draft!)
const tab = ref<'colors' | 'background' | 'css'>('colors')
const tabs = ['colors', 'background', 'css'] as const
const collapsed = ref(false)
const modes = ['dark', 'light'] as const
const mode = computed<SchemeMode>(() => appearance.mode)
function preview(value: SchemeMode) {
  themes.previewMode = value
}

const label = (id: string) => {
  const scheme = schemes.find((s) => s.id === id)!
  const family = schemeFamilies.find((f) => f.id === scheme.family)!.name
  return scheme.name === family ? family : `${family} ${scheme.name}`
}
const variants = computed(() => schemes.filter((s) => s.mode === mode.value))
const seeds = computed(() => draft.value.colors[mode.value])
function customize() {
  draft.value.colors[mode.value] = seedsOf(schemeFor(draft.value.base[mode.value], mode.value))
}
function useBase() {
  delete draft.value.colors[mode.value]
}
/** Text and accent colors that schemeTokens moved to keep text readable. */
const nudged = computed(() => {
  const value = seeds.value
  if (!value) return false
  const tokens = schemeTokens(themeScheme(draft.value, mode.value))
  return tokens.text !== value.ink || tokens.muted !== value.muted || tokens.accent !== value.accent
})

// ---- Background ---------------------------------------------------------------------------------

const imageTypes = computed(() =>
  (chat.data?.capabilities?.upload?.types ?? []).filter((type) => type.startsWith('image/')),
)
const canUpload = computed(() => imageTypes.value.length > 0)
const uploading = ref(false)
const uploadError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const background = computed(() => draft.value.background)
function setBackground(changes: Partial<ThemeBackground>) {
  draft.value.background = { ...(draft.value.background ?? defaultBackground()), ...changes }
}
async function tones(file: File) {
  try {
    const bitmap = await createImageBitmap(file, { resizeWidth: 24, resizeHeight: 24 })
    const canvas = document.createElement('canvas')
    canvas.width = 24
    canvas.height = 24
    const context = canvas.getContext('2d')!
    context.drawImage(bitmap, 0, 0)
    bitmap.close()
    return measureTones(context.getImageData(0, 0, 24, 24).data)
  } catch {
    return null
  }
}
async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploadError.value = ''
  const limits = chat.data?.capabilities?.upload
  if (!file.type.startsWith('image/')) {
    uploadError.value = t('themes.studio.notImage')
    return
  }
  if (limits && file.size > limits.maxBytes) {
    uploadError.value = t('themes.studio.tooBig')
    return
  }
  uploading.value = true
  try {
    const measured = await tones(file)
    const uploaded = await apiClient().upload(file, chat.networkId ?? '')
    const path = instanceFilePath(uploaded.url, location.origin)
    if (!path) throw new Error(t('themes.studio.notInstance'))
    setBackground({ url: path, tones: measured })
  } catch (error) {
    uploadError.value = errorText(error)
  } finally {
    uploading.value = false
  }
}
const floors = computed(() =>
  backgroundFloors(appearance.tokens, draft.value.background?.tones ?? null),
)
const thumb = computed(() =>
  background.value?.url ? { backgroundImage: `url("${background.value.url}")` } : {},
)
const number = (event: Event) => Number((event.target as HTMLInputElement).value)

// ---- CSS --------------------------------------------------------------------------------------

const report = computed(() => themes.filter(draft.value.css).report)
const removals = computed(() => cssRemovalSummary(report.value, t))
const kib = computed(() => Math.ceil(new TextEncoder().encode(draft.value.css).length / 1024))
const safeAddress = computed(() => `${location.origin}/?safe`)

function save() {
  void themes.saveDraft()
}
</script>

<template>
  <section
    :class="['studio', { collapsed }]"
    role="dialog"
    :aria-label="$t('themes.studio.title')"
  >
    <header>
      <h2>{{ themes.studioNew ? $t('themes.studio.newTitle') : $t('themes.studio.title') }}</h2>
      <button
        type="button"
        class="quiet collapse"
        :aria-expanded="!collapsed"
        @click="collapsed = !collapsed"
      >
        {{ collapsed ? $t('themes.studio.expand') : $t('themes.studio.collapse') }}
      </button>
    </header>
    <div
      class="tabs"
      role="tablist"
    >
      <button
        v-for="item in tabs"
        :id="`theme-tab-${item}`"
        :key="item"
        type="button"
        role="tab"
        :aria-selected="tab === item"
        :aria-controls="`theme-panel-${item}`"
        @click="tab = item"
      >
        {{ $t(`themes.studio.tabs.${item}`) }}
      </button>
    </div>
    <div
      :id="`theme-panel-${tab}`"
      class="body"
      role="tabpanel"
      :aria-labelledby="`theme-tab-${tab}`"
    >
      <p
        v-if="themes.restorable"
        class="note"
      >
        {{ $t('themes.studio.restoreNote') }}
        <span class="row">
          <button
            type="button"
            @click="themes.restoreDraft()"
          >
            {{ $t('themes.studio.restore') }}
          </button>
          <button
            type="button"
            class="quiet"
            @click="themes.restorable = null"
          >
            {{ $t('themes.dismiss') }}
          </button>
        </span>
      </p>
      <p
        v-if="themes.conflict"
        class="note warn"
        role="alert"
      >
        {{ $t('themes.studio.conflict') }}
        <span class="row">
          <button
            type="button"
            @click="themes.saveDraft(true)"
          >
            {{ $t('themes.studio.keepMine') }}
          </button>
          <button
            type="button"
            @click="themes.loadTheirs()"
          >
            {{ $t('themes.studio.loadTheirs') }}
          </button>
        </span>
      </p>
      <label class="field">
        <span>{{ $t('themes.studio.name') }}</span>
        <input
          v-model="draft.name"
          type="text"
          :maxlength="themeLimits.name"
        />
      </label>

      <template v-if="tab === 'colors'">
        <div class="field">
          <span class="label">{{ $t('themes.studio.mode') }}</span>
          <span
            class="segmented"
            role="group"
            :aria-label="$t('themes.studio.mode')"
          >
            <button
              v-for="item in modes"
              :key="item"
              type="button"
              :aria-pressed="mode === item"
              @click="preview(item)"
            >
              {{ $t(`appearance.modes.${item}`) }}
            </button>
          </span>
          <small>{{ $t('themes.studio.modeHelp') }}</small>
        </div>
        <label class="field">
          <span>{{ $t('themes.studio.base') }}</span>
          <select
            :value="draft.base[mode]"
            @change="draft.base[mode] = ($event.target as HTMLSelectElement).value"
          >
            <option
              v-for="scheme in variants"
              :key="scheme.id"
              :value="scheme.id"
            >
              {{ label(scheme.id) }}
            </option>
          </select>
          <small>{{ $t('themes.studio.baseHelp') }}</small>
        </label>
        <template v-if="seeds">
          <div class="swatches">
            <label
              v-for="key in seedKeys"
              :key="key"
              class="swatch"
            >
              <input
                v-model="seeds[key]"
                type="color"
              />{{ $t(`themes.studio.seeds.${key}`) }}
            </label>
          </div>
          <span class="label">{{ $t('themes.studio.palette') }}</span>
          <div class="swatches">
            <label
              v-for="key in paletteKeys"
              :key="key"
              class="swatch"
            >
              <input
                v-model="seeds.palette[key]"
                type="color"
              />{{ $t(`themes.studio.paletteNames.${key}`) }}
            </label>
          </div>
          <p
            v-if="nudged"
            class="note"
          >
            {{ $t('themes.studio.nudged') }}
          </p>
          <span class="row">
            <button
              type="button"
              @click="useBase"
            >
              {{ $t('themes.studio.useBase') }}
            </button>
          </span>
        </template>
        <p
          v-else
          class="note"
        >
          {{ $t('themes.studio.baseOnly', { scheme: label(draft.base[mode]) }) }}
          <span class="row">
            <button
              type="button"
              class="primary"
              @click="customize"
            >
              {{ $t('themes.studio.customize') }}
            </button>
          </span>
        </p>
      </template>

      <template v-else-if="tab === 'background'">
        <p
          v-if="!canUpload"
          class="note"
        >
          {{ $t('themes.studio.noUploads') }}
        </p>
        <template v-else>
          <p
            v-if="background && !background.url"
            class="note"
          >
            {{ $t('themes.studio.needsImage') }}
          </p>
          <div
            v-if="background?.url"
            class="thumb"
            :style="thumb"
            role="img"
            :aria-label="$t('themes.studio.currentImage')"
          />
          <span class="row">
            <button
              type="button"
              :disabled="uploading"
              @click="fileInput?.click()"
            >
              {{
                uploading
                  ? $t('themes.studio.uploading')
                  : background?.url
                    ? $t('themes.studio.replaceImage')
                    : $t('themes.studio.chooseImage')
              }}
            </button>
            <button
              v-if="background"
              type="button"
              class="danger"
              @click="draft.background = null"
            >
              {{ $t('themes.studio.removeImage') }}
            </button>
            <input
              ref="fileInput"
              type="file"
              class="sr-only"
              tabindex="-1"
              :accept="imageTypes.join(',')"
              :aria-label="$t('themes.studio.chooseImage')"
              @change="chooseImage"
            />
          </span>
          <p
            v-if="uploadError"
            class="error"
            role="alert"
          >
            {{ uploadError }}
          </p>
          <small class="hint">{{ $t('themes.studio.imageHelp') }}</small>
          <template v-if="background">
            <label class="field">
              <span>{{ $t('themes.studio.placement') }}</span>
              <select
                :value="background.placement"
                @change="
                  setBackground({
                    placement: ($event.target as HTMLSelectElement)
                      .value as ThemeBackground['placement'],
                  })
                "
              >
                <option
                  v-for="item in backgroundOptions.placement"
                  :key="item"
                  :value="item"
                >
                  {{ $t(`themes.studio.placements.${item}`) }}
                </option>
              </select>
            </label>
            <div class="row">
              <label class="field">
                <span>{{ $t('themes.studio.fit') }}</span>
                <select
                  :value="background.fit"
                  @change="
                    setBackground({
                      fit: ($event.target as HTMLSelectElement).value as ThemeBackground['fit'],
                    })
                  "
                >
                  <option
                    v-for="item in backgroundOptions.fit"
                    :key="item"
                    :value="item"
                  >
                    {{ $t(`themes.studio.fits.${item}`) }}
                  </option>
                </select>
              </label>
              <label class="field">
                <span>{{ $t('themes.studio.position') }}</span>
                <select
                  :value="background.position"
                  @change="
                    setBackground({
                      position: ($event.target as HTMLSelectElement)
                        .value as ThemeBackground['position'],
                    })
                  "
                >
                  <option
                    v-for="item in backgroundOptions.position"
                    :key="item"
                    :value="item"
                  >
                    {{ $t(`themes.studio.positions.${item}`) }}
                  </option>
                </select>
              </label>
            </div>
            <label class="field">
              <span>{{ $t('themes.studio.dim') }}</span>
              <span class="row">
                <input
                  type="range"
                  :min="floors.dim"
                  :max="backgroundRanges.dim.max"
                  step="5"
                  :value="Math.max(background.dim, floors.dim)"
                  @input="setBackground({ dim: number($event) })"
                />
                <output>{{ Math.max(background.dim, floors.dim) }}%</output>
              </span>
              <small>{{ $t('themes.studio.dimHelp', { floor: floors.dim }) }}</small>
            </label>
            <label class="field">
              <span>{{ $t('themes.studio.blur') }}</span>
              <span class="row">
                <input
                  type="range"
                  :min="backgroundRanges.blur.min"
                  :max="backgroundRanges.blur.max"
                  :value="background.blur"
                  @input="setBackground({ blur: number($event) })"
                />
                <output>{{ background.blur }} px</output>
              </span>
            </label>
            <label
              v-if="background.placement === 'app'"
              class="field"
            >
              <span>{{ $t('themes.studio.panels') }}</span>
              <span class="row">
                <input
                  type="range"
                  :min="floors.panelOpacity"
                  :max="backgroundRanges.panelOpacity.max"
                  step="5"
                  :value="Math.max(background.panelOpacity, floors.panelOpacity)"
                  @input="setBackground({ panelOpacity: number($event) })"
                />
                <output>{{ Math.max(background.panelOpacity, floors.panelOpacity) }}%</output>
              </span>
              <small>{{ $t('themes.studio.panelsHelp', { floor: floors.panelOpacity }) }}</small>
            </label>
          </template>
        </template>
      </template>

      <template v-else>
        <p
          v-if="!themes.cssAllowed"
          class="note warn"
        >
          {{ themes.operatorCss ? $t('themes.customCssOff') : $t('themes.customCssOperatorOff') }}
        </p>
        <label class="field">
          <span>{{ $t('themes.studio.css') }}</span>
          <textarea
            v-model="draft.css"
            spellcheck="false"
            autocapitalize="off"
            autocomplete="off"
            :maxlength="themeLimits.css"
          />
          <small>{{
            $t('themes.studio.cssSize', { used: kib, limit: themeLimits.css / 1024 })
          }}</small>
        </label>
        <p
          v-if="removals.length"
          class="note warn"
          role="status"
        >
          {{ $t('themes.studio.removedTitle') }}
        </p>
        <ul
          v-if="removals.length"
          class="report"
        >
          <li
            v-for="line in removals"
            :key="line"
          >
            {{ line }}
          </li>
        </ul>
        <small class="hint">{{ $t('themes.studio.cssHelp') }}</small>
        <small class="hint">{{ $t('themes.safeHelp', { address: safeAddress }) }}</small>
      </template>
      <p
        v-if="themes.saveError"
        class="error"
        role="alert"
      >
        {{ themes.saveError }}
      </p>
    </div>
    <footer>
      <label class="toggle">
        <input
          v-model="themes.compare"
          type="checkbox"
        />{{ $t('themes.studio.compare') }}
      </label>
      <span class="spacer" />
      <button
        type="button"
        @click="themes.closeStudio()"
      >
        {{ $t('themes.studio.discard') }}
      </button>
      <button
        type="button"
        class="primary"
        :disabled="themes.saving || uploading"
        @click="save"
      >
        {{ themes.saving ? $t('common.saving') : $t('themes.studio.save') }}
      </button>
    </footer>
  </section>
</template>
