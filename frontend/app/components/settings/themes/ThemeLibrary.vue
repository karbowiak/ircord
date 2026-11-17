<script setup vapor lang="ts">
/**
 * Settings → Appearance → Your themes: the account's theme library (#88). Apply, edit, duplicate,
 * export and delete themes, import one from a file or pasted text, and the "Use custom CSS"
 * switch. Editing happens in the theme studio beside the live app (ThemeLayer).
 */
import {
  themeLimits,
  themeScheme,
  themeSupported,
  newerVersion,
  type Theme,
} from '../../../../shared/themes'
import type { CssReport } from '../../../../shared/theme-css'
import { cssRemovalSummary } from './css-report'

const themes = useThemesStore()
const appearance = useAppearanceStore()
const actions = useWorkspaceActions()
const { confirm } = usePrompt()
const { t } = useI18n()
const $t = t

const fileInput = ref<HTMLInputElement | null>(null)
const pasting = ref(false)
const pasted = ref('')
const busy = ref('')
const error = ref('')
/** The theme just imported, shown with its summary until dismissed. */
const imported = shallowRef<{ theme: Theme; report: CssReport } | null>(null)
const safeAddress = computed(() => `${location.origin}/?safe`)

const preview = (theme: Theme) => {
  const scheme = themeScheme(theme, appearance.mode)
  return {
    '--pv-canvas': scheme.canvas,
    '--pv-surface': scheme.surface,
    '--pv-overlay': scheme.overlay,
    '--pv-ink': scheme.ink,
    '--pv-muted': scheme.muted,
    '--pv-accent': scheme.accent,
  }
}
const parts = (theme: Theme) =>
  [
    (theme.colors.dark || theme.colors.light) && t('themes.parts.colors'),
    theme.background && t('themes.parts.background'),
    theme.css.trim() && t('themes.parts.css'),
  ]
    .filter(Boolean)
    .join(' · ') || t('themes.parts.none')

/** "Check your theme after this update", once per release, for a kept theme with CSS. */
const noticeKey = 'ircord.theme.update-note'
const seenNotice = ref(readLocal(noticeKey, (v) => (typeof v === 'string' ? v : '')))
const updateNotice = computed(
  () =>
    !!themes.kept?.css.trim() &&
    newerVersion(themes.version, themes.kept.madeWith) &&
    seenNotice.value !== themes.version,
)
function dismissNotice() {
  seenNotice.value = themes.version
  writeLocal(noticeKey, themes.version)
}

async function run(label: string, task: () => Promise<unknown>) {
  busy.value = label
  error.value = ''
  try {
    await task()
  } catch (failure) {
    error.value = errorText(failure)
  } finally {
    busy.value = ''
  }
}

function apply(theme: Theme) {
  // A theme with CSS runs on trial; close Settings so its CSS is visible while you decide.
  if (themes.apply(theme) === 'trial') actions.closeOverlay()
}
function edit(theme: Theme) {
  themes.edit(theme)
  actions.closeOverlay()
}
function create() {
  themes.create(t('themes.newName'))
  actions.closeOverlay()
}
const duplicate = (theme: Theme) =>
  run('duplicate', () => themes.duplicate(theme, t('themes.copyName', { name: theme.name })))
async function remove(theme: Theme) {
  if (
    await confirm({
      title: t('themes.deleteTitle', { name: theme.name }),
      body:
        appearance.settings.theme === theme.id
          ? t('themes.deleteBodyActive')
          : t('themes.deleteBody'),
      confirm: t('themes.delete'),
      danger: true,
    })
  )
    await run('delete', () => themes.remove(theme))
}

async function importText(raw: string) {
  await run('import', async () => {
    const result = await themes.importFrom(raw)
    if (!result.ok) {
      error.value = t(`themes.importErrors.${result.reason}`, { size: themeLimits.css / 1024 })
      return
    }
    imported.value = { theme: result.theme, report: result.report }
    pasting.value = false
    pasted.value = ''
  })
}
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > themeLimits.importText) {
    error.value = t('themes.importErrors.too-large')
    return
  }
  await importText(await file.text())
}
const importSummary = computed(() => {
  const value = imported.value
  if (!value) return []
  const { theme, report } = value
  const modes = (['dark', 'light'] as const).filter((mode) => theme.colors[mode])
  const lines = theme.css.trim() ? theme.css.trim().split('\n').length : 0
  return [
    modes.length
      ? t('themes.summary.colors', {
          modes: modes.map((mode) => t(`appearance.modes.${mode}`)).join(t('themes.summary.and')),
        })
      : t('themes.summary.noColors'),
    theme.background ? t('themes.summary.background') : t('themes.summary.noBackground'),
    lines ? t('themes.summary.css', { lines }, lines) : t('themes.summary.noCss'),
    ...cssRemovalSummary(report, t),
  ]
})
</script>

<template>
  <SettingGroup
    :title="$t('themes.title')"
    :scope="$t('themes.scope')"
  >
    <div
      data-setting="your-themes"
      class="theme-library grid gap-3"
    >
      <p
        v-if="updateNotice"
        class="theme-note m-0 flex items-start gap-2 rounded-md p-2.5 text-[length:var(--ui-small)] bg-[color-mix(in_srgb,var(--yellow)_12%,transparent)]"
        role="status"
      >
        <AppIcon
          name="info"
          :size="15"
        /><span class="flex-1">{{ $t('themes.updateNotice') }}</span
        ><UiButton
          variant="ghost"
          size="xs"
          @click="dismissNotice"
          >{{ $t('themes.dismiss') }}</UiButton
        >
      </p>
      <div
        v-if="themes.themes.length"
        class="theme-grid grid [grid-template-columns:repeat(auto-fill,_minmax(260px,_1fr))] gap-2.5"
        role="list"
      >
        <div
          v-for="theme in themes.themes"
          :key="theme.id"
          role="listitem"
          :class="[
            'theme-card grid gap-2 rounded-[var(--radius)] border border-border bg-[var(--panel)] p-2',
            { 'is-active': appearance.settings.theme === theme.id },
          ]"
          :style="preview(theme)"
        >
          <span
            class="scheme-preview flex gap-[5px] h-14 p-1.5 bg-[var(--pv-canvas)] rounded-[calc(var(--radius)_-_3px)] overflow-hidden"
            aria-hidden="true"
          >
            <span class="scheme-preview-rail grid gap-1 [align-content:start]"><i /><i /></span>
            <span
              class="scheme-preview-main flex-1 grid gap-[5px] [align-content:start] p-[7px] bg-[var(--pv-surface)] rounded-[5px]"
            >
              <span
                class="scheme-preview-line h-[5px] rounded-[3px] bg-[var(--pv-muted)] opacity-55"
              />
              <span
                class="scheme-preview-line h-[5px] rounded-[3px] bg-[var(--pv-muted)] opacity-55"
              />
              <span class="scheme-preview-pill w-[40%] h-2 rounded-[4px] bg-[var(--pv-accent)]" />
            </span>
          </span>
          <span class="grid gap-0.5 px-0.5">
            <strong class="flex items-center gap-1 truncate"
              >{{ theme.name
              }}<AppIcon
                v-if="appearance.settings.theme === theme.id"
                name="check"
                :size="13"
                :label="$t('themes.inUse')"
            /></strong>
            <small class="text-muted-foreground">{{
              themeSupported(theme) ? parts(theme) : $t('themes.newer')
            }}</small>
          </span>
          <span class="theme-actions flex items-center gap-1 flex-wrap">
            <UiButton
              v-if="appearance.settings.theme !== theme.id"
              size="xs"
              :disabled="!themeSupported(theme)"
              @click="apply(theme)"
              >{{ $t('themes.apply') }}</UiButton
            >
            <UiButton
              v-else
              size="xs"
              variant="secondary"
              @click="themes.useBuiltIn()"
              >{{ $t('themes.useBuiltIn') }}</UiButton
            >
            <UiButton
              variant="ghost"
              size="icon-xs"
              :disabled="!themeSupported(theme)"
              :aria-label="$t('themes.editNamed', { name: theme.name })"
              :title="$t('themes.edit')"
              @click="edit(theme)"
              ><AppIcon
                name="pencil"
                :size="13"
            /></UiButton>
            <UiButton
              variant="ghost"
              size="icon-xs"
              :disabled="!!busy || themes.themes.length >= themeLimits.themes"
              :aria-label="$t('themes.duplicateNamed', { name: theme.name })"
              :title="$t('themes.duplicate')"
              @click="duplicate(theme)"
              ><AppIcon
                name="copy"
                :size="13"
            /></UiButton>
            <UiButton
              variant="ghost"
              size="icon-xs"
              :aria-label="$t('themes.exportNamed', { name: theme.name })"
              :title="$t('themes.export')"
              @click="themes.download(theme)"
              ><AppIcon
                name="download"
                :size="13"
            /></UiButton>
            <UiButton
              variant="ghost"
              size="icon-xs"
              :disabled="!!busy"
              :aria-label="$t('themes.deleteNamed', { name: theme.name })"
              :title="$t('themes.delete')"
              @click="remove(theme)"
              ><AppIcon
                name="trash"
                :size="13"
            /></UiButton>
          </span>
        </div>
      </div>
      <p
        v-else-if="!themes.loading"
        class="m-0 text-[length:var(--ui-small)] text-muted-foreground"
      >
        {{ $t('themes.empty') }}
      </p>

      <div class="flex flex-wrap items-center gap-2">
        <UiButton
          size="sm"
          :disabled="themes.themes.length >= themeLimits.themes"
          @click="create"
          ><AppIcon
            name="plus"
            :size="14"
          />{{ $t('themes.new') }}</UiButton
        >
        <UiButton
          size="sm"
          variant="secondary"
          :disabled="!!busy || themes.themes.length >= themeLimits.themes"
          @click="fileInput?.click()"
          ><AppIcon
            name="upload"
            :size="14"
          />{{ $t('themes.importFile') }}</UiButton
        >
        <UiButton
          size="sm"
          variant="secondary"
          :disabled="!!busy || themes.themes.length >= themeLimits.themes"
          :aria-expanded="pasting"
          @click="pasting = !pasting"
          >{{ $t('themes.importPaste') }}</UiButton
        >
        <input
          ref="fileInput"
          type="file"
          accept=".json,application/json"
          class="sr-only"
          tabindex="-1"
          :aria-label="$t('themes.importFile')"
          @change="importFile"
        />
      </div>
      <div
        v-if="pasting"
        class="grid gap-2"
      >
        <AppField
          v-slot="control"
          :label="$t('themes.pasteLabel')"
        >
          <UiTextarea
            v-bind="control"
            v-model="pasted"
            rows="5"
            class="font-code text-[12px]"
            spellcheck="false"
          />
        </AppField>
        <div class="flex gap-2">
          <UiButton
            size="sm"
            :disabled="!pasted.trim() || !!busy"
            @click="importText(pasted)"
            >{{ $t('themes.importAction') }}</UiButton
          >
          <UiButton
            size="sm"
            variant="ghost"
            @click="pasting = false"
            >{{ $t('common.cancel') }}</UiButton
          >
        </div>
      </div>
      <p
        v-if="error"
        class="m-0 text-[length:var(--ui-small)] text-destructive"
        role="alert"
      >
        {{ error }}
      </p>

      <section
        v-if="imported"
        class="theme-import grid gap-2 rounded-[var(--radius)] border border-border p-3"
        :aria-label="$t('themes.importedTitle', { name: imported.theme.name })"
      >
        <strong>{{ $t('themes.importedTitle', { name: imported.theme.name }) }}</strong>
        <ul class="m-0 grid gap-1 pl-4 text-[length:var(--ui-small)]">
          <li
            v-for="line in importSummary"
            :key="line"
          >
            {{ line }}
          </li>
        </ul>
        <p
          v-if="imported.theme.css.trim()"
          class="m-0 text-[length:var(--ui-small)] text-muted-foreground"
        >
          {{ $t('themes.importWarning') }}
        </p>
        <details v-if="imported.theme.css.trim()">
          <summary class="cursor-pointer text-[length:var(--ui-small)]">
            {{ $t('themes.showCss') }}
          </summary>
          <pre
            class="mt-2 max-h-60 overflow-auto rounded-md bg-[var(--input)] p-2 text-[12px] font-code whitespace-pre-wrap"
            >{{ imported.theme.css }}</pre>
        </details>
        <div class="flex gap-2">
          <UiButton
            size="sm"
            @click="(apply(imported.theme), (imported = null))"
            >{{ $t('themes.apply') }}</UiButton
          >
          <UiButton
            size="sm"
            variant="ghost"
            @click="imported = null"
            >{{ $t('themes.done') }}</UiButton
          >
        </div>
      </section>
    </div>

    <SettingRow
      data-setting="custom-css"
      :label="$t('themes.customCss')"
      :description="
        !themes.operatorCss
          ? $t('themes.customCssOperatorOff')
          : themes.accountCss
            ? $t('themes.customCssHelp')
            : $t('themes.customCssOff')
      "
    >
      <AppSwitch
        :model-value="themes.accountCss && themes.operatorCss"
        :disabled="!themes.operatorCss || !themes.library || !!busy"
        :aria-label="$t('themes.customCss')"
        @change="run('custom-css', () => themes.setCustomCss($event))"
      />
    </SettingRow>
    <p class="m-0 text-[length:var(--ui-caption)] text-muted-foreground">
      {{ $t('themes.safeHelp', { address: safeAddress }) }}
    </p>
  </SettingGroup>
</template>
