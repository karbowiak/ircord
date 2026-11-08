<script setup lang="ts">
/** Settings → Appearance: theme, accent, interface and motion. Message layout is under Messages. */
import {
  accentColors,
  appearanceDefaults,
  interfaceAppearanceKeys,
  interfaceFonts,
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
  <div class="appearance-settings">
    <div
      data-setting="appearance-sync"
      class="sync-row"
    >
      <AppIcon
        :name="appearance.sync ? 'cloud' : 'cloudOff'"
        :size="18"
      />
      <span class="setting-copy"
        ><strong>Same look on every device</strong><small role="status">{{ status }}</small
        ><small
          >Covers this section, plus message layout and animated emoji under Messages &amp;
          media.</small
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
      <div
        data-setting="colour-scheme"
        class="scheme-grid"
        role="radiogroup"
        aria-label="Colour scheme"
      >
        <button
          v-for="family in families"
          :key="family.id"
          :class="['scheme-card', { active: activeScheme.family === family.id }]"
          :style="previewStyle(family.preview)"
          role="radio"
          :aria-checked="activeScheme.family === family.id"
          @click="chooseFamily(family.id)"
        >
          <span
            class="scheme-preview"
            aria-hidden="true"
          >
            <span class="scheme-preview-rail"><i /><i /></span>
            <span class="scheme-preview-main">
              <span class="scheme-preview-line ink" />
              <span class="scheme-preview-line" />
              <span class="scheme-preview-pill" />
            </span>
          </span>
          <span class="scheme-label">
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
        </button>
      </div>
      <div
        data-setting="scheme-variants"
        class="setting-row variant-row"
      >
        <AppField
          v-slot="control"
          label="Dark variant"
          class="variant-field"
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
          class="variant-field"
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
          class="scheme-source"
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
        <div
        data-setting="interface-font"
          class="accent-swatches"
          role="radiogroup"
          aria-label="Accent colour"
        >
          <button
            :class="['accent-swatch', { active: settings.accent === 'scheme' }]"
            :style="{ '--swatch': activeScheme.accent }"
            role="radio"
            :aria-checked="settings.accent === 'scheme'"
            title="Theme accent"
            @click="update({ accent: 'scheme' })"
          >
            <AppIcon
              :name="settings.accent === 'scheme' ? 'check' : 'palette'"
              :size="14"
            /><span class="sr-only">Theme accent</span>
          </button>
          <span class="accent-divider" />
          <button
            v-for="preset in presets"
            :key="preset"
            :class="['accent-swatch', { active: settings.accent === preset }]"
            :style="{ '--swatch': accentColors[preset] }"
            role="radio"
            :aria-checked="settings.accent === preset"
            :title="preset"
            @click="update({ accent: preset })"
          >
            <AppIcon
              v-if="settings.accent === preset"
              name="check"
              :size="14"
            /><span class="sr-only">{{ preset }}</span>
          </button>
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
        </div>
      </SettingRow>
    </SettingGroup>

    <SettingGroup title="Interface">
      <SettingRow
        label="Typeface"
        description="Menus, lists and buttons. The message font is under Messages & media."
        stacked
      >
        <div
        data-setting="sidebar-density"
          class="font-grid"
          role="radiogroup"
          aria-label="Interface typeface"
        >
          <button
            v-for="[value, font] in interfaceFontOptions"
            :key="value"
            :class="['choice-card', 'font-card', { active: settings.font === value }]"
            :style="{ fontFamily: font.stack }"
            role="radio"
            :aria-checked="settings.font === value"
            @click="update({ font: value })"
          >
            <span class="font-glyph">Aa</span>
            <strong>{{ font.label }}</strong>
            <small>{{ font.note }}</small>
          </button>
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
          even where <SettingLink to="messages">Messages &amp; media</SettingLink> lets them play.
          System follows your device’s setting.</template
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
      class="settings-footer"
    >
      <button
        class="button secondary small"
        @click="update(appearanceDefaults(interfaceAppearanceKeys))"
      >
        <AppIcon
          name="reset"
          :size="14"
        />Reset appearance
      </button>
    </div>
  </div>
</template>
