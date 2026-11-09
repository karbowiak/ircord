<script setup lang="ts">
/**
 * Message layout follows appearance sync; channel activity is an account preference.
 * Ignored people follow the account’s notification rules.
 */
import {
  appearanceDefaults,
  messageAppearanceKeys,
  messageFonts,
  ranges,
  type Appearance,
} from '../../shared/appearance'
import { scopeLabels } from '../../shared/settings-index'
const appearance = useAppearanceStore()
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)

const layouts = [
  { value: 'cozy', label: 'Cozy', note: 'Avatars and grouped messages' },
  { value: 'compact', label: 'Compact', note: 'Smaller avatars, tighter rows' },
  { value: 'irc', label: 'IRC', note: 'One line per message' },
] as const
// A real chat line, with Danish æ/ø/å, a symbol and an emoji to show each font's fallbacks.
const messageFontSample = 'Ses vi på fredag? Æbler, øl & kaffe → 9:30 ☕'
const messageFontOptions = Object.entries(messageFonts) as [
  Appearance['messageFont'],
  (typeof messageFonts)[Appearance['messageFont']],
][]
const layoutScope = computed(() => (appearance.sync ? scopeLabels.account : scopeLabels.device))
</script>

<template>
  <div class="appearance-settings grid gap-6">
    <MessagePreview />

    <SettingGroup title="Layout">
      <template #scope
        >{{ layoutScope }} ·
        <SettingLink to="appearance">Change in Appearance</SettingLink></template
      >
      <div data-setting="message-layout">
        <UiRadioGroup
          :model-value="settings.messageLayout"
          class="layout-choices grid [grid-template-columns:repeat(3,_1fr)] gap-2.5"
          aria-label="Message layout"
          @update:model-value="update({ messageLayout: $event as Appearance['messageLayout'] })"
        >
          <UiRadioGroupItem
            v-for="item in layouts"
            :key="item.value"
            variant="card"
            :value="item.value"
            class="choice-card"
          >
            <span
              class="layout-glyph grid gap-[5px] p-2.5 h-16 [align-content:center] rounded-[calc(var(--radius)_-_3px)] bg-background"
              :class="`glyph-${item.value}`"
              aria-hidden="true"
              ><i /><i /><i
            /></span>
            <strong>{{ item.label }}</strong>
            <small>{{ item.note }}</small>
          </UiRadioGroupItem>
        </UiRadioGroup>
      </div>
      <SettingRow
        data-setting="message-font"
        label="Message font"
        stacked
      >
        <div data-setting="text-size">
          <UiRadioGroup
            :model-value="settings.messageFont"
            class="font-grid grid [grid-template-columns:repeat(auto-fill,_minmax(150px,_1fr))] gap-2"
            aria-label="Message font"
            @update:model-value="update({ messageFont: $event as Appearance['messageFont'] })"
          >
            <UiRadioGroupItem
              v-for="[value, font] in messageFontOptions"
              :key="value"
              variant="card"
              :value="value"
              class="choice-card font-card"
            >
              <span
                class="font-sample text-[15px] leading-[1.4] mb-1 text-body"
                :style="{ fontFamily: font.stack }"
                >{{ messageFontSample }}</span
              >
              <strong>{{ font.label }}</strong>
              <small>{{ font.note }}</small>
            </UiRadioGroupItem>
          </UiRadioGroup>
        </div>
      </SettingRow>
      <SettingRow
        data-setting="text-size"
        label="Message text size"
        description="Changes chat messages and the composer. Interface scale is under Appearance."
      >
        <div class="range-control flex items-center gap-3 w-[min(100%,_300px)]">
          <UiSlider
            :min="ranges.messageSize.min"
            :max="ranges.messageSize.max"
            :step="1"
            :model-value="[settings.messageSize]"
            label="Message text size"
            @update:model-value="(value) => value && update({ messageSize: value[0] })"
          /><output class="min-w-9 text-right text-[12px] text-muted-foreground tabular-nums"
            >{{ settings.messageSize }}px</output
          >
        </div>
      </SettingRow>
      <SettingRow
        data-setting="group-spacing"
        label="Space between groups"
        description="Gap before each new speaker."
      >
        <div class="range-control flex items-center gap-3 w-[min(100%,_300px)]">
          <UiSlider
            :min="ranges.groupSpacing.min"
            :max="ranges.groupSpacing.max"
            :step="2"
            :model-value="[settings.groupSpacing]"
            label="Space between message groups"
            @update:model-value="(value) => value && update({ groupSpacing: value[0] })"
          /><output class="min-w-9 text-right text-[12px] text-muted-foreground tabular-nums"
            >{{ settings.groupSpacing }}px</output
          >
        </div>
      </SettingRow>
      <SettingRow
        data-setting="clock"
        label="Clock"
      >
        <AppSegmented
          :model-value="settings.timeFormat"
          :options="[
            { value: '24h', label: '24-hour' },
            { value: '12h', label: '12-hour' },
          ]"
          label="Time format"
          @update:model-value="update({ timeFormat: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="coloured-names"
        label="Coloured names"
        description="A stable colour for each person."
      >
        <AppSwitch
          :model-value="settings.coloredNames"
          aria-label="Coloured names"
          @update:model-value="update({ coloredNames: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup
      title="Conversation"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="channel-activity"
        label="Channel activity"
        description="Joins, parts, quits and nick changes as small activity lines. Kicks, bans and topic changes remain visible; duplicate history notices go to Server."
      >
        <AppSegmented
          :model-value="preferences.preferences.events"
          :options="[
            { value: 'collapse', label: 'Compact' },
            { value: 'hide', label: 'Hide' },
          ]"
          label="Join and part messages"
          @update:model-value="preferences.update({ events: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup title="Animation">
      <SettingRow
        data-setting="animate-media"
        label="Play GIFs and animated emoji"
        :description="
          appearance.reducedMotion
            ? 'Reduce motion is on, so emoji and picker GIFs show a still frame.'
            : 'Custom emoji everywhere and GIFs in the pickers. When off, a still frame is shown.'
        "
      >
        <AppSwitch
          :model-value="settings.animateMedia"
          aria-label="Play GIFs and animated emoji"
          @update:model-value="update({ animateMedia: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup
      title="Hidden messages"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="ignored-people"
        label="Ignored people"
        stacked
      >
        <template #description
          >Their messages are hidden and never notify. A nick, or a mask like
          <code>*!*@host.example</code>.</template
        >
        <ChipInput
          :items="notifications.settings.ignore"
          :add="
            (value) => !!notifications.update({ ignore: [...notifications.settings.ignore, value] })
          "
          :remove="
            (value) =>
              notifications.update({
                ignore: notifications.settings.ignore.filter((x) => x !== value),
              })
          "
          :remove-label="(value) => `Stop ignoring ${value}`"
          aria-label="Ignore a nick or mask"
          placeholder="Add a nick or mask, press Enter"
          maxlength="120"
        />
      </SettingRow>
    </SettingGroup>

    <div
      data-setting="messages-reset"
      class="settings-footer flex justify-end pt-2 border-t [border-top-style:solid] border-t-border"
    >
      <UiButton
        variant="secondary"
        size="sm"
        @click="update(appearanceDefaults(messageAppearanceKeys))"
      >
        <AppIcon
          name="reset"
          :size="14"
        />Reset message layout
      </UiButton>
    </div>
  </div>
</template>
