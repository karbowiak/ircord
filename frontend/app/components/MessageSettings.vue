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
const { t } = useI18n()
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)

const layouts = computed(() => [
  { value: 'cozy', label: t('messages.layout.cozy'), note: t('messages.layout.cozyNote') },
  { value: 'compact', label: t('messages.layout.compact'), note: t('messages.layout.compactNote') },
  { value: 'irc', label: t('messages.layout.irc'), note: t('messages.layout.ircNote') },
] as const)
// A real chat line, with Danish æ/ø/å, a symbol and an emoji to show each font's fallbacks.
const messageFontSample = 'Ses vi på fredag? Æbler, øl & kaffe → 9:30 ☕'
const messageFontOptions = Object.entries(messageFonts) as [
  Appearance['messageFont'],
  (typeof messageFonts)[Appearance['messageFont']],
][]
const layoutScope = computed(() =>
  t(appearance.sync ? scopeLabels.account : scopeLabels.device),
)
</script>

<template>
  <div class="appearance-settings grid gap-6">
    <MessagePreview />

    <SettingGroup :title="$t('messages.groups.layout')">
      <template #scope
        >{{ layoutScope }} ·
        <SettingLink to="appearance">{{ $t('messages.changeInAppearance') }}</SettingLink></template
      >
      <div data-setting="message-layout">
        <UiRadioGroup
          :model-value="settings.messageLayout"
          class="layout-choices grid [grid-template-columns:repeat(3,_1fr)] gap-2.5"
          :aria-label="$t('messages.messageLayoutLabel')"
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
        :label="$t('messages.messageFontLabel')"
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
        :label="$t('messages.textSize')"
        :description="$t('messages.textSizeHelp')"
      >
        <div class="range-control flex items-center gap-3 w-[min(100%,_300px)]">
          <UiSlider
            :min="ranges.messageSize.min"
            :max="ranges.messageSize.max"
            :step="1"
            :model-value="[settings.messageSize]"
            :label="$t('messages.textSize')"
            @update:model-value="(value) => value && update({ messageSize: value[0] })"
          /><output class="min-w-9 text-right text-[12px] text-muted-foreground tabular-nums"
            >{{ settings.messageSize }}px</output
          >
        </div>
      </SettingRow>
      <SettingRow
        data-setting="group-spacing"
        :label="$t('messages.groupSpacing')"
        :description="$t('messages.groupSpacingHelp')"
      >
        <div class="range-control flex items-center gap-3 w-[min(100%,_300px)]">
          <UiSlider
            :min="ranges.groupSpacing.min"
            :max="ranges.groupSpacing.max"
            :step="2"
            :model-value="[settings.groupSpacing]"
            :label="$t('messages.groupSpacingSlider')"
            @update:model-value="(value) => value && update({ groupSpacing: value[0] })"
          /><output class="min-w-9 text-right text-[12px] text-muted-foreground tabular-nums"
            >{{ settings.groupSpacing }}px</output
          >
        </div>
      </SettingRow>
      <SettingRow
        data-setting="clock"
        :label="$t('messages.clock')"
      >
        <AppSegmented
          :model-value="settings.timeFormat"
          :options="[
            { value: '24h', label: $t('messages.clock24') },
            { value: '12h', label: $t('messages.clock12') },
          ]"
          :label="$t('messages.timeFormat')"
          @update:model-value="update({ timeFormat: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="coloured-names"
        :label="$t('messages.colouredNames')"
        :description="$t('messages.colouredNamesHelp')"
      >
        <AppSwitch
          :model-value="settings.coloredNames"
          :aria-label="$t('messages.colouredNames')"
          @update:model-value="update({ coloredNames: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup
      :title="$t('messages.groups.conversation')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingRow
        data-setting="channel-activity"
        :label="$t('messages.channelActivity')"
        :description="$t('messages.channelActivityHelp')"
      >
        <AppSegmented
          :model-value="preferences.preferences.events"
          :options="[
            { value: 'collapse', label: $t('messages.events.compact') },
            { value: 'hide', label: $t('messages.events.hide') },
          ]"
          :label="$t('messages.joinPart')"
          @update:model-value="preferences.update({ events: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup :title="$t('messages.groups.animation')">
      <SettingRow
        data-setting="animate-media"
        :label="$t('messages.animateMedia')"
        :description="
          appearance.reducedMotion ? $t('messages.animateReduced') : $t('messages.animateOn')
        "
      >
        <AppSwitch
          :model-value="settings.animateMedia"
          :aria-label="$t('messages.animateMedia')"
          @update:model-value="update({ animateMedia: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup
      :title="$t('messages.groups.hidden')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingRow
        data-setting="ignored-people"
        :label="$t('messages.ignoredPeople')"
        stacked
      >
        <template #description>
          <i18n-t keypath="messages.ignoredHelp">
            <template #mask><code>*!*@host.example</code></template>
          </i18n-t>
        </template>
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
          :remove-label="(value) => $t('messages.stopIgnoring', { value })"
          :aria-label="$t('messages.ignoreLabel')"
          :placeholder="$t('messages.ignorePlaceholder')"
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
        />{{ $t('messages.resetLayout') }}
      </UiButton>
    </div>
  </div>
</template>
