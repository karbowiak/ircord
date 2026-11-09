<script setup lang="ts">
/** Settings → Appearance: theme, accent, interface and motion. Message layout is under Messages. */
import {
  accentColors,
  appearanceDefaults,
  interfaceAppearanceKeys,
  interfaceFonts,
  ranges,
  type AccentPreset,
  type Appearance,
} from '../../shared/appearance'
import {
  schemeFamilies,
  schemeFor,
  schemes,
  type Scheme,
  type SchemeMode,
} from '../../shared/schemes'
const appearance = useAppearanceStore()
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)

const modes = [
  { value: 'system', label: 'System', icon: 'monitor' },
  { value: 'dark', label: 'Dark', icon: 'moon' },
  { value: 'light', label: 'Light', icon: 'sun' },
] as const
const presets = Object.keys(accentColors) as AccentPreset[]
const interfaceFontOptions = Object.entries(interfaceFonts) as [
  Appearance['font'],
  (typeof interfaceFonts)[Appearance['font']],
][]

const activeScheme = computed(() =>
  schemeFor(
    appearance.mode === 'dark' ? settings.value.schemeDark : settings.value.schemeLight,
    appearance.mode,
  ),
)
const families = computed(() =>
  schemeFamilies.map((family) => ({
    ...family,
    preview: family.variants.find((v) => v.mode === appearance.mode) ?? family.variants[0]!,
    modes: [...new Set(family.variants.map((v) => v.mode))],
  })),
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
    ? 'Saved on this device only'
    : appearance.saveState === 'saving'
      ? 'Saving…'
      : appearance.saveState === 'error'
        ? 'Couldn’t save to your account; kept on this device'
        : 'Synced to your account',
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
        ><strong>Same look on every device</strong><small role="status">{{ status }}</small
        ><small
          >Covers this section, plus message layout and animated emoji under Chat.</small
        ></span
      >
      <AppSwitch
        :model-value="appearance.sync"
        aria-label="Sync appearance across devices"
        @update:model-value="appearance.setSync($event)"
      />
    </div>

    <MessagePreview />

    <SettingGroup title="Theme">
      <SettingRow
        data-setting="theme-mode"
        label="Mode"
      >
        <AppSegmented
          :model-value="settings.mode"
          :options="modes"
          label="Colour mode"
          @update:model-value="update({ mode: $event })"
        />
      </SettingRow>
      <div data-setting="colour-scheme">
        <UiRadioGroup
          :model-value="activeScheme.family"
          @update:model-value="chooseFamily(String($event))"
          class="scheme-grid grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2.5"
          aria-label="Colour scheme"
        >
          <UiRadioGroupItem
            variant="card"
            :value="family.id"
            v-for="family in families"
            :key="family.id"
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
                  ? `${family.modes[0] === 'dark' ? 'Dark' : 'Light'} only`
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
          label="Dark variant"
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
          label="Light variant"
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
          >Palette source<AppIcon
            name="external"
            :size="12"
        /></a>
      </div>
      <SettingRow
        data-setting="accent"
        label="Accent"
        description="Selection, mentions and buttons. Adjusted to stay readable."
        stacked
      >
        <div data-setting="interface-font">
          <UiRadioGroup
            :model-value="settings.accent"
            @update:model-value="update({ accent: $event as Appearance['accent'] })"
            class="accent-swatches flex flex-wrap gap-2 items-center"
            aria-label="Accent colour"
          >
            <UiRadioGroupItem
              variant="swatch"
              :value="'scheme'"
              :class="['accent-swatch', { active: settings.accent === 'scheme' }]"
              :style="{ '--swatch': activeScheme.accent }"
              title="Theme accent"
            >
              <AppIcon
                :name="settings.accent === 'scheme' ? 'check' : 'palette'"
                :size="14"
              /><span class="sr-only">Theme accent</span>
            </UiRadioGroupItem>
            <span class="accent-divider w-[1px] h-5 bg-border" />
            <UiRadioGroupItem
              variant="swatch"
              :value="preset"
              v-for="preset in presets"
              :key="preset"
              :class="['accent-swatch', { active: settings.accent === preset }]"
              :style="{ '--swatch': accentColors[preset] }"
              :title="preset"
            >
              <AppIcon
                v-if="settings.accent === preset"
                name="check"
                :size="14"
              /><span class="sr-only">{{ preset }}</span>
            </UiRadioGroupItem>
            <label
              :class="['accent-swatch', 'custom', { active: settings.accent === 'custom' }]"
              :style="{ '--swatch': customColor }"
              title="Custom colour"
            >
              <AppIcon
                v-if="settings.accent === 'custom'"
                name="check"
                :size="14"
              /><input
                type="color"
                class="sr-only"
                aria-label="Custom accent colour"
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

    <SettingGroup title="Interface">
      <SettingRow
        label="Typeface"
        description="Menus, lists and buttons. The message font is under Chat."
        stacked
      >
        <div data-setting="sidebar-density">
          <UiRadioGroup
            :model-value="settings.font"
            @update:model-value="update({ font: $event as Appearance['font'] })"
            class="font-grid grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2"
            aria-label="Interface typeface"
          >
            <UiRadioGroupItem
              variant="card"
              :value="value"
              v-for="[value, font] in interfaceFontOptions"
              :key="value"
              class="choice-card font-card"
              :style="{ fontFamily: font.stack }"
            >
              <span class="font-glyph text-[26px] font-[600] leading-[1.2]">Aa</span>
              <strong>{{ font.label }}</strong>
              <small>{{ font.note }}</small>
            </UiRadioGroupItem>
          </UiRadioGroup>
        </div>
      </SettingRow>
      <SettingRow
        data-setting="interface-scale"
        label="Interface scale"
        description="Scale navigation, menus and controls. 100% uses a 14px base. Set message text size under Chat."
      >
        <div class="range-control flex w-[min(100%,300px)] items-center gap-3">
          <UiSlider
            :min="ranges.interfaceScale.min"
            :max="ranges.interfaceScale.max"
            :step="5"
            :model-value="[settings.interfaceScale]"
            label="Interface scale"
            @update:model-value="(value) => value && update({ interfaceScale: value[0] })"
          />
          <output
            class="min-w-10 text-right text-[length:var(--ui-small)] text-muted-foreground tabular-nums"
            >{{ settings.interfaceScale }}%</output
          >
        </div>
      </SettingRow>
      <SettingRow
        label="Sidebar density"
        description="Row height in channel and member lists."
      >
        <AppSegmented
          :model-value="settings.density"
          :options="[
            { value: 'comfortable', label: 'Comfortable' },
            { value: 'compact', label: 'Compact' },
          ]"
          label="Sidebar density"
          @update:model-value="update({ density: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="corners"
        label="Corners"
      >
        <AppSegmented
          :model-value="settings.corners"
          :options="[
            { value: 'rounded', label: 'Rounded' },
            { value: 'soft', label: 'Soft' },
            { value: 'square', label: 'Square' },
          ]"
          label="Corners"
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

    <SettingGroup title="Motion">
      <SettingRow
        data-setting="reduce-motion"
        label="Reduce motion"
      >
        <template #description
          >Turns off transitions and animations, and keeps GIFs, videos and animated emoji still
          even where <SettingLink to="messages">Chat</SettingLink> lets them play. System follows
          your device’s setting.</template
        >
        <AppSegmented
          :model-value="settings.motion"
          :options="[
            { value: 'system', label: 'System' },
            { value: 'reduced', label: 'On' },
            { value: 'full', label: 'Off' },
          ]"
          label="Reduce motion"
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
        />Reset appearance
      </UiButton>
    </div>
  </div>
</template>
