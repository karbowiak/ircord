<script setup vapor lang="ts">
/** Settings → Appearance: theme, accent, interface and motion. Message layout is under Messages. */
import {
  accentColors,
  appearanceDefaults,
  interfaceAppearanceKeys,
  interfaceFonts,
  ranges,
  type AccentPreset,
  type Appearance,
} from '../../../shared/appearance'
import {
  schemeFamilies,
  schemeFor,
  schemes,
  type Scheme,
  type SchemeMode,
} from '../../../shared/schemes'
const appearance = useAppearanceStore()
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)
const { t, n: $n } = useI18n()
const $t = t

const modes = computed(
  () =>
    [
      { value: 'system', label: t('appearance.modes.system'), icon: 'monitor' },
      { value: 'dark', label: t('appearance.modes.dark'), icon: 'moon' },
      { value: 'light', label: t('appearance.modes.light'), icon: 'sun' },
    ] as const,
)
const presets = Object.keys(accentColors) as AccentPreset[]
const interfaceFontOptions = Object.entries(interfaceFonts) as [
  Appearance['font'],
  (typeof interfaceFonts)[Appearance['font']],
][]
const schemeDescriptionKeys: Record<string, string> = {
  ircord: 'ircord',
  hibiki: 'hibiki',
  midnight: 'midnight',
  catppuccin: 'catppuccin',
  'tokyo-night': 'tokyo-night',
  nord: 'nord',
  'rose-pine': 'rose-pine',
  gruvbox: 'gruvbox',
  dracula: 'dracula',
  solarized: 'solarized',
  everforest: 'everforest',
  kanagawa: 'kanagawa',
  github: 'github',
}
const interfaceFontNote = (value: Appearance['font']) => t(`mediaUi.interfaceFontNotes.${value}`)

const activeScheme = computed(() =>
  schemeFor(
    appearance.mode === 'dark' ? settings.value.schemeDark : settings.value.schemeLight,
    appearance.mode,
  ),
)
const families = computed(() =>
  schemeFamilies.map((family) => {
    const description = schemeDescriptionKeys[family.id]
    return {
      ...family,
      description: description
        ? t(`mediaUi.appearance.schemeDescriptions.${description}`)
        : family.description,
      preview: family.variants.find((v) => v.mode === appearance.mode) ?? family.variants[0]!,
      modes: [...new Set(family.variants.map((v) => v.mode))],
    }
  }),
)
const label = (scheme: Scheme) => {
  const family = schemeFamilies.find((f) => f.id === scheme.family)!.name
  return scheme.name === family ? family : `${family} ${scheme.name}`
}
const variants = (mode: SchemeMode) =>
  schemes.filter((s) => s.mode === mode).map((s) => ({ value: s.id, label: label(s) }))
const darkVariants = variants('dark')
const lightVariants = variants('light')
function chooseFamily(id: string) {
  const found = schemes.filter((s) => s.family === id)
  const dark = found.find((s) => s.mode === 'dark')
  const light = found.find((s) => s.mode === 'light')
  update({
    schemeDark: dark?.id ?? settings.value.schemeDark,
    schemeLight: light?.id ?? settings.value.schemeLight,
    // Single-mode families switch the mode so the choice is visible.
    ...(!dark ? { mode: 'light' as const } : !light ? { mode: 'dark' as const } : {}),
  })
}
const previewStyle = (scheme: Scheme) => ({
  '--pv-canvas': scheme.canvas,
  '--pv-surface': scheme.surface,
  '--pv-overlay': scheme.overlay,
  '--pv-ink': scheme.ink,
  '--pv-muted': scheme.muted,
  '--pv-accent': scheme.accent,
})
const customColor = computed(() => settings.value.accentCustom || '#a78bfa')
const status = computed(() =>
  !appearance.sync
    ? t('appearance.sync.localOnly')
    : appearance.saveState === 'saving'
      ? t('common.saving')
      : appearance.saveState === 'error'
        ? t('appearance.sync.saveFailed')
        : t('appearance.sync.synced'),
)
</script>

<template>
  <div class="appearance-settings grid gap-6">
    <div
      data-setting="appearance-sync"
      class="sync-row flex items-center gap-3 py-3 px-3.5 rounded-md bg-card text-muted-foreground"
    >
      <AppIcon
        :name="appearance.sync ? 'cloud' : 'cloudOff'"
        :size="18"
      />
      <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
        ><strong>{{ $t('appearance.sync.title') }}</strong
        ><small role="status">{{ status }}</small
        ><small>{{ $t('appearance.sync.help') }}</small></span
      >
      <AppSwitch
        :model-value="appearance.sync"
        :aria-label="$t('appearance.sync.aria')"
        @update:model-value="appearance.setSync($event)"
      />
    </div>

    <MessagePreview />

    <SettingGroup :title="$t('appearance.groups.theme')">
      <SettingRow
        data-setting="theme-mode"
        :label="$t('appearance.mode')"
      >
        <AppSegmented
          :model-value="settings.mode"
          :options="modes"
          :label="$t('appearance.colourMode')"
          @update:model-value="update({ mode: $event })"
        />
      </SettingRow>
      <div data-setting="colour-scheme">
        <UiRadioGroup
          :model-value="activeScheme.family"
          class="scheme-grid grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5"
          :aria-label="$t('appearance.colourScheme')"
          @update:model-value="chooseFamily(String($event))"
        >
          <UiRadioGroupItem
            v-for="family in families"
            :key="family.id"
            variant="card"
            :value="family.id"
            class="scheme-card"
            :style="previewStyle(family.preview)"
          >
            <span
              class="scheme-preview flex gap-[5px] h-16 p-1.5 bg-[var(--pv-canvas)] rounded-[calc(var(--radius)_-_3px)] overflow-hidden"
              aria-hidden="true"
            >
              <span class="scheme-preview-rail grid gap-1 [align-content:start]"><i /><i /></span>
              <span
                class="scheme-preview-main flex-1 grid gap-[5px] [align-content:start] p-[7px] bg-[var(--pv-surface)] rounded-[5px]"
              >
                <span
                  class="scheme-preview-line h-[5px] rounded-[3px] bg-[var(--pv-muted)] opacity-55 ink"
                />
                <span
                  class="scheme-preview-line h-[5px] rounded-[3px] bg-[var(--pv-muted)] opacity-55"
                />
                <span class="scheme-preview-pill w-[40%] h-2 rounded-[4px] bg-[var(--pv-accent)]" />
              </span>
            </span>
            <span class="scheme-label grid pt-0 pr-0.5 pb-0.5 pl-0.5">
              <strong
                >{{ family.name
                }}<AppIcon
                  v-if="activeScheme.family === family.id"
                  name="check"
                  :size="13"
              /></strong>
              <small>{{
                family.modes.length === 1
                  ? family.modes[0] === 'dark'
                    ? $t('appearance.onlyDark')
                    : $t('appearance.onlyLight')
                  : family.description
              }}</small>
            </span>
          </UiRadioGroupItem>
        </UiRadioGroup>
      </div>
      <div
        data-setting="scheme-variants"
        class="setting-row variant-row flex items-center gap-4 justify-start flex-wrap"
      >
        <AppField
          v-slot="control"
          :label="$t('appearance.darkVariant')"
          class="variant-field w-55 max-w-full"
        >
          <AppSelect
            v-bind="control"
            :model-value="settings.schemeDark"
            :options="darkVariants"
            @change="update({ schemeDark: $event })"
          />
        </AppField>
        <AppField
          v-slot="control"
          :label="$t('appearance.lightVariant')"
          class="variant-field w-55 max-w-full"
        >
          <AppSelect
            v-bind="control"
            :model-value="settings.schemeLight"
            :options="lightVariants"
            @change="update({ schemeLight: $event })"
          />
        </AppField>
        <a
          v-if="activeScheme.source"
          :href="activeScheme.source"
          target="_blank"
          rel="noopener noreferrer"
          class="scheme-source inline-flex items-center gap-1 self-end ml-auto pb-2 text-[12px] text-muted-foreground"
          >{{ $t('appearance.paletteSource')
          }}<AppIcon
            name="external"
            :size="12"
        /></a>
      </div>
      <SettingRow
        data-setting="accent"
        :label="$t('appearance.accent')"
        :description="$t('appearance.accentHelp')"
        stacked
      >
        <div data-setting="interface-font">
          <UiRadioGroup
            :model-value="settings.accent"
            class="accent-swatches flex flex-wrap gap-2 items-center"
            :aria-label="$t('appearance.accentAria')"
            @update:model-value="update({ accent: $event as Appearance['accent'] })"
          >
            <UiRadioGroupItem
              variant="swatch"
              :value="'scheme'"
              :class="['accent-swatch', { active: settings.accent === 'scheme' }]"
              :style="{ '--swatch': activeScheme.accent }"
              :title="$t('appearance.themeAccent')"
            >
              <AppIcon
                :name="settings.accent === 'scheme' ? 'check' : 'palette'"
                :size="14"
              /><span class="sr-only">{{ $t('appearance.themeAccent') }}</span>
            </UiRadioGroupItem>
            <span class="accent-divider w-[1px] h-5 bg-border" />
            <UiRadioGroupItem
              v-for="preset in presets"
              :key="preset"
              variant="swatch"
              :value="preset"
              :class="['accent-swatch', { active: settings.accent === preset }]"
              :style="{ '--swatch': accentColors[preset] }"
              :title="$t(`appearance.accentNames.${preset}`)"
            >
              <AppIcon
                v-if="settings.accent === preset"
                name="check"
                :size="14"
              /><span class="sr-only">{{ $t(`appearance.accentNames.${preset}`) }}</span>
            </UiRadioGroupItem>
            <label
              :class="['accent-swatch', 'custom', { active: settings.accent === 'custom' }]"
              :style="{ '--swatch': customColor }"
              :title="$t('appearance.customColour')"
            >
              <AppIcon
                v-if="settings.accent === 'custom'"
                name="check"
                :size="14"
              /><input
                type="color"
                class="sr-only"
                :aria-label="$t('appearance.customAccentAria')"
                :value="customColor"
                @input="
                  update({
                    accent: 'custom',
                    accentCustom: ($event.target as HTMLInputElement).value,
                  })
                "
              />
            </label>
          </UiRadioGroup>
        </div>
      </SettingRow>
    </SettingGroup>

    <SettingGroup :title="$t('appearance.groups.interface')">
      <SettingRow
        :label="$t('appearance.typeface')"
        :description="$t('appearance.typefaceHelp')"
        stacked
      >
        <div data-setting="sidebar-density">
          <UiRadioGroup
            :model-value="settings.font"
            class="font-grid grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2"
            :aria-label="$t('appearance.typefaceAria')"
            @update:model-value="update({ font: $event as Appearance['font'] })"
          >
            <UiRadioGroupItem
              v-for="[value, font] in interfaceFontOptions"
              :key="value"
              variant="card"
              :value="value"
              class="choice-card font-card"
              :style="{ fontFamily: font.stack }"
            >
              <span class="font-glyph text-[26px] font-[600] leading-[1.2]">Aa</span>
              <strong>{{ font.label }}</strong>
              <small>{{ interfaceFontNote(value) }}</small>
            </UiRadioGroupItem>
          </UiRadioGroup>
        </div>
      </SettingRow>
      <SettingRow
        data-setting="interface-scale"
        :label="$t('appearance.interfaceScale')"
        :description="$t('appearance.interfaceScaleHelp')"
      >
        <div class="range-control flex w-[min(100%,300px)] items-center gap-3">
          <UiSlider
            :min="ranges.interfaceScale.min"
            :max="ranges.interfaceScale.max"
            :step="5"
            :model-value="[settings.interfaceScale]"
            :label="$t('appearance.interfaceScale')"
            @update:model-value="(value) => value && update({ interfaceScale: value[0] })"
          />
          <output
            class="min-w-10 text-right text-[length:var(--ui-small)] text-muted-foreground tabular-nums"
            >{{
              $n(settings.interfaceScale / 100, {
                style: 'percent',
                maximumFractionDigits: 0,
              })
            }}</output
          >
        </div>
      </SettingRow>
      <SettingRow
        :label="$t('appearance.density')"
        :description="$t('appearance.densityHelp')"
      >
        <AppSegmented
          :model-value="settings.density"
          :options="[
            { value: 'comfortable', label: $t('appearance.comfortable') },
            { value: 'compact', label: $t('appearance.compact') },
          ]"
          :label="$t('appearance.density')"
          @update:model-value="update({ density: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="corners"
        :label="$t('appearance.corners')"
      >
        <AppSegmented
          :model-value="settings.corners"
          :options="[
            { value: 'rounded', label: $t('appearance.rounded') },
            { value: 'soft', label: $t('appearance.soft') },
            { value: 'square', label: $t('appearance.square') },
          ]"
          :label="$t('appearance.corners')"
          @update:model-value="update({ corners: $event })"
        >
          <template #option="{ option }"
            ><span
              :class="['corner-glyph', `corner-${option.value}`]"
              aria-hidden="true"
            />{{ option.label }}</template
          >
        </AppSegmented>
      </SettingRow>
    </SettingGroup>

    <SettingGroup :title="$t('appearance.groups.motion')">
      <SettingRow
        data-setting="reduce-motion"
        :label="$t('appearance.reduceMotion')"
      >
        <template #description>
          <i18n-t keypath="appearance.reduceMotionHelp">
            <template #link><SettingLink to="messages" /></template>
          </i18n-t>
        </template>
        <AppSegmented
          :model-value="settings.motion"
          :options="[
            { value: 'system', label: $t('appearance.motion.system') },
            { value: 'reduced', label: $t('appearance.motion.on') },
            { value: 'full', label: $t('appearance.motion.off') },
          ]"
          :label="$t('appearance.reduceMotion')"
          @update:model-value="update({ motion: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <div
      data-setting="appearance-reset"
      class="settings-footer flex justify-end pt-2 border-t [border-top-style:solid] border-t-border"
    >
      <UiButton
        variant="secondary"
        size="sm"
        @click="update(appearanceDefaults(interfaceAppearanceKeys))"
      >
        <AppIcon
          name="reset"
          :size="14"
        />{{ $t('appearance.reset') }}
      </UiButton>
    </div>
  </div>
</template>
