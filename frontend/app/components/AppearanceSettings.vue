<script setup lang="ts">
import {
  accentColors,
  interfaceFonts,
  messageFonts,
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
import { memberNameColor } from '../../shared/utils/identity'
const appearance = useAppearanceStore()
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)

const modes = [
  { value: 'system', label: 'System', icon: 'monitor' },
  { value: 'dark', label: 'Dark', icon: 'moon' },
  { value: 'light', label: 'Light', icon: 'sun' },
] as const
const layouts = [
  { value: 'cozy', label: 'Cozy', note: 'Avatars and grouped messages' },
  { value: 'compact', label: 'Compact', note: 'Smaller avatars, tighter rows' },
  { value: 'irc', label: 'IRC', note: 'One line per message' },
] as const
const presets = Object.keys(accentColors) as AccentPreset[]
const interfaceFontOptions = Object.entries(interfaceFonts) as [
  Appearance['font'],
  (typeof interfaceFonts)[Appearance['font']],
][]
const messageFontOptions = Object.entries(messageFonts) as [
  Appearance['messageFont'],
  (typeof messageFonts)[Appearance['messageFont']],
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
const variants = (mode: SchemeMode) => schemes.filter((s) => s.mode === mode)
const label = (scheme: Scheme) => {
  const family = schemeFamilies.find((f) => f.id === scheme.family)!.name
  return scheme.name === family ? family : `${family} ${scheme.name}`
}
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
const sampleName = (id: string) => ({
  color: settings.value.coloredNames
    ? memberNameColor({ id, account: id }, appearance.mode, appearance.tokens.main)
    : 'var(--text)',
})
const sampleTime = (minutes: number) =>
  new Date(2026, 0, 1, 9, minutes).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    hour12: settings.value.timeFormat === '12h',
  })
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
        ><strong>Same look on every device</strong><small role="status">{{ status }}</small></span
      >
      <button
        role="switch"
        class="toggle"
        :class="{ on: appearance.sync }"
        :aria-checked="appearance.sync"
        aria-label="Sync appearance across devices"
        @click="appearance.setSync(!appearance.sync)"
      >
        <span />
      </button>
    </div>

    <div
      class="appearance-preview"
      aria-hidden="true"
    >
      <div
        class="preview-row"
        :data-layout="settings.messageLayout"
      >
        <template v-if="settings.messageLayout === 'irc'">
          <span class="preview-line"
            ><time>{{ sampleTime(41) }}</time
            ><b :style="sampleName('jules')">jules</b
            ><span>anyone up for a code review?</span></span
          >
          <span class="preview-line"
            ><time>{{ sampleTime(42) }}</time
            ><b :style="sampleName('noah')">noah</b
            ><span>sure, send the link <a>#development</a></span></span
          >
        </template>
        <template v-else>
          <span class="preview-message"
            ><i class="preview-avatar tone-mint">J</i
            ><span
              ><span class="preview-meta"
                ><b :style="sampleName('jules')">Jules</b><time>{{ sampleTime(41) }}</time></span
              ><span class="preview-text">anyone up for a code review?</span></span
            ></span
          >
          <span class="preview-message"
            ><i class="preview-avatar tone-blue">N</i
            ><span
              ><span class="preview-meta"
                ><b :style="sampleName('noah')">Noah</b><time>{{ sampleTime(42) }}</time></span
              ><span class="preview-text">sure, send the link <a>#development</a></span></span
            ></span
          >
        </template>
      </div>
    </div>

    <section class="setting-group">
      <h3>Theme</h3>
      <div
        data-setting="theme-mode"
        class="setting-row"
      >
        <span class="setting-copy"><strong>Mode</strong></span>
        <div
          class="segmented"
          role="group"
          aria-label="Colour mode"
        >
          <button
            v-for="mode in modes"
            :key="mode.value"
            :class="{ active: settings.mode === mode.value }"
            :aria-pressed="settings.mode === mode.value"
            @click="update({ mode: mode.value })"
          >
            <AppIcon
              :name="mode.icon"
              :size="14"
            />{{ mode.label }}
          </button>
        </div>
      </div>
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
        <label
          >Dark variant<select
            :value="settings.schemeDark"
            @change="update({ schemeDark: ($event.target as HTMLSelectElement).value })"
          >
            <option
              v-for="s in variants('dark')"
              :key="s.id"
              :value="s.id"
            >
              {{ label(s) }}
            </option>
          </select></label
        ><label
          >Light variant<select
            :value="settings.schemeLight"
            @change="update({ schemeLight: ($event.target as HTMLSelectElement).value })"
          >
            <option
              v-for="s in variants('light')"
              :key="s.id"
              :value="s.id"
            >
              {{ label(s) }}
            </option>
          </select></label
        >
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
      <div
        data-setting="accent"
        class="setting-row stacked"
      >
        <span class="setting-copy"
          ><strong>Accent</strong
          ><small>Selection, mentions and buttons. Adjusted to stay readable.</small></span
        >
        <div
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
      </div>
    </section>

    <section class="setting-group">
      <h3>Messages</h3>
      <div
        data-setting="message-layout"
        class="layout-choices"
        role="radiogroup"
        aria-label="Message layout"
      >
        <button
          v-for="item in layouts"
          :key="item.value"
          :class="['choice-card', { active: settings.messageLayout === item.value }]"
          role="radio"
          :aria-checked="settings.messageLayout === item.value"
          @click="update({ messageLayout: item.value })"
        >
          <span
            class="layout-glyph"
            :class="`glyph-${item.value}`"
            aria-hidden="true"
            ><i /><i /><i
          /></span>
          <strong>{{ item.label }}</strong>
          <small>{{ item.note }}</small>
        </button>
      </div>
      <div
        data-setting="message-font"
        class="setting-row stacked"
      >
        <span class="setting-copy"><strong>Message font</strong></span>
        <div
          class="font-grid"
          role="radiogroup"
          aria-label="Message font"
        >
          <button
            v-for="[value, font] in messageFontOptions"
            :key="value"
            :class="['choice-card', 'font-card', { active: settings.messageFont === value }]"
            role="radio"
            :aria-checked="settings.messageFont === value"
            @click="update({ messageFont: value })"
          >
            <span
              class="font-sample"
              :style="{ fontFamily: font.stack }"
              >The quick brown fox</span
            >
            <strong>{{ font.label }}</strong>
            <small>{{ font.note }}</small>
          </button>
        </div>
      </div>
      <div
        data-setting="text-size"
        class="setting-row"
      >
        <span class="setting-copy"><strong>Text size</strong></span>
        <div class="range-control">
          <input
            type="range"
            :min="ranges.messageSize.min"
            :max="ranges.messageSize.max"
            step="1"
            :value="settings.messageSize"
            aria-label="Message text size"
            @input="update({ messageSize: Number(($event.target as HTMLInputElement).value) })"
          /><output>{{ settings.messageSize }}px</output>
        </div>
      </div>
      <div
        data-setting="group-spacing"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Space between groups</strong><small>Gap before each new speaker.</small></span
        >
        <div class="range-control">
          <input
            type="range"
            :min="ranges.groupSpacing.min"
            :max="ranges.groupSpacing.max"
            step="2"
            :value="settings.groupSpacing"
            aria-label="Space between message groups"
            @input="update({ groupSpacing: Number(($event.target as HTMLInputElement).value) })"
          /><output>{{ settings.groupSpacing }}px</output>
        </div>
      </div>
      <div
        data-setting="clock"
        class="setting-row"
      >
        <span class="setting-copy"><strong>Clock</strong></span>
        <div
          class="segmented"
          role="group"
          aria-label="Time format"
        >
          <button
            :class="{ active: settings.timeFormat === '24h' }"
            :aria-pressed="settings.timeFormat === '24h'"
            @click="update({ timeFormat: '24h' })"
          >
            24-hour
          </button>
          <button
            :class="{ active: settings.timeFormat === '12h' }"
            :aria-pressed="settings.timeFormat === '12h'"
            @click="update({ timeFormat: '12h' })"
          >
            12-hour
          </button>
        </div>
      </div>
      <div
        data-setting="coloured-names"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Coloured names</strong><small>A stable colour for each person.</small></span
        >
        <button
          role="switch"
          class="toggle"
          :class="{ on: settings.coloredNames }"
          :aria-checked="settings.coloredNames"
          aria-label="Coloured names"
          @click="update({ coloredNames: !settings.coloredNames })"
        >
          <span />
        </button>
      </div>
    </section>

    <section class="setting-group">
      <h3>Interface</h3>
      <div
        data-setting="interface-font"
        class="setting-row stacked"
      >
        <span class="setting-copy"><strong>Typeface</strong></span>
        <div
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
      </div>
      <div
        data-setting="sidebar-density"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Sidebar density</strong
          ><small>Row height in channel and member lists.</small></span
        >
        <div
          class="segmented"
          role="group"
          aria-label="Sidebar density"
        >
          <button
            v-for="d in ['comfortable', 'compact'] as const"
            :key="d"
            :class="{ active: settings.density === d }"
            :aria-pressed="settings.density === d"
            @click="update({ density: d })"
          >
            {{ d === 'comfortable' ? 'Comfortable' : 'Compact' }}
          </button>
        </div>
      </div>
      <div
        data-setting="corners"
        class="setting-row"
      >
        <span class="setting-copy"><strong>Corners</strong></span>
        <div
          class="segmented"
          role="group"
          aria-label="Corners"
        >
          <button
            v-for="c in ['rounded', 'soft', 'square'] as const"
            :key="c"
            :class="{ active: settings.corners === c }"
            :aria-pressed="settings.corners === c"
            @click="update({ corners: c })"
          >
            <span
              :class="['corner-glyph', `corner-${c}`]"
              aria-hidden="true"
            />{{ c[0]!.toUpperCase() + c.slice(1) }}
          </button>
        </div>
      </div>
    </section>

    <section class="setting-group">
      <h3>Motion</h3>
      <div
        data-setting="animate-media"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Play GIFs and animated emoji</strong
          ><small
            >In the picker and emoji. When off, a still frame is shown. GIFs and videos in messages
            follow “Autoplay GIFs and videos” under Notifications.</small
          ></span
        >
        <button
          role="switch"
          class="toggle"
          :class="{ on: settings.animateMedia }"
          :aria-checked="settings.animateMedia"
          aria-label="Play GIFs and animated emoji"
          @click="update({ animateMedia: !settings.animateMedia })"
        >
          <span />
        </button>
      </div>
      <div
        data-setting="reduce-motion"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Reduce motion</strong><small>Transitions, animations and autoplay.</small></span
        >
        <div
          class="segmented"
          role="group"
          aria-label="Reduce motion"
        >
          <button
            v-for="m in [
              { value: 'system', label: 'System' },
              { value: 'reduced', label: 'On' },
              { value: 'full', label: 'Off' },
            ] as const"
            :key="m.value"
            :class="{ active: settings.motion === m.value }"
            :aria-pressed="settings.motion === m.value"
            @click="update({ motion: m.value })"
          >
            {{ m.label }}
          </button>
        </div>
      </div>
    </section>

    <div
      data-setting="appearance-reset"
      class="settings-footer"
    >
      <button
        class="button secondary small"
        @click="appearance.reset()"
      >
        <AppIcon
          name="reset"
          :size="14"
        />Reset to defaults
      </button>
    </div>
  </div>
</template>
