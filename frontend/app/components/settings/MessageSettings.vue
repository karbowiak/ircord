<script setup vapor lang="ts">
/** Message layout follows appearance sync; channel activity is an account preference. */
import {
  appearanceDefaults,
  messageAppearanceKeys,
  messageFonts,
  ranges,
  type Appearance,
} from '../../../shared/appearance'
import { scopeLabels } from '../../../shared/settings-index'
import { startupChannels } from '../../../shared/utils/startup'
import { channelTarget } from '../../../shared/utils/irc'
const appearance = useAppearanceStore()
const preferences = usePreferencesStore()
const chat = useChatStore()
const { t, n } = useI18n()
const $t = t
const settings = computed(() => appearance.settings)
const update = (changes: Partial<Appearance>) => appearance.update(changes)

const layouts = computed(
  () =>
    [
      { value: 'cozy', label: t('messages.layout.cozy'), note: t('messages.layout.cozyNote') },
      {
        value: 'compact',
        label: t('messages.layout.compact'),
        note: t('messages.layout.compactNote'),
      },
      { value: 'irc', label: t('messages.layout.irc'), note: t('messages.layout.ircNote') },
    ] as const,
)
// A real chat line, with Danish æ/ø/å, a symbol and an emoji to show each font's fallbacks.
const messageFontSample = 'Ses vi på fredag? Æbler, øl & kaffe → 9:30 ☕'
const messageFontOptions = Object.entries(messageFonts) as [
  Appearance['messageFont'],
  (typeof messageFonts)[Appearance['messageFont']],
][]
const fontNote = (value: Appearance['messageFont']) =>
  t(`mediaUi.messageSettings.fontNotes.${value}`)
const startupChannelsAvailable = computed(() => (chat.data ? startupChannels(chat.data) : []))
const startupOptions = computed(() =>
  startupChannelsAvailable.value.map((channel) => ({
    value: channel.id,
    label: `${channelTarget(channel)} · ${chat.data?.networks.find((network) => network.id === channel.networkId)?.name ?? ''}`,
  })),
)
const startupChannel = computed(
  () =>
    preferences.preferences.startup.channelId ??
    startupChannelsAvailable.value.find(
      (channel) => channelTarget(channel).toLowerCase() === '#general',
    )?.id ??
    null,
)
function setStartupMode(mode: 'last' | 'channel') {
  preferences.update({
    startup: {
      mode,
      channelId:
        preferences.preferences.startup.channelId ??
        (mode === 'channel' ? startupChannel.value : null),
    },
  })
}
function setStartupChannel(channelId: string | null) {
  if (channelId) preferences.update({ startup: { mode: 'channel', channelId } })
}
</script>

<template>
  <div class="appearance-settings grid gap-6">
    <MessagePreview />

    <SettingGroup
      :title="$t('messages.groups.layout')"
      :scope="appearance.sync ? undefined : $t(scopeLabels.device)"
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
            :aria-label="$t('messages.messageFontLabel')"
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
              <strong>{{
                value === 'interface' ? $t('mediaUi.messageSettings.interfaceFont') : font.label
              }}</strong>
              <small>{{ fontNote(value) }}</small>
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
            >{{ n(settings.messageSize) }}px</output
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
            >{{ n(settings.groupSpacing) }}px</output
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

    <SettingGroup :title="$t('messages.groups.conversation')">
      <SettingRow
        data-setting="startup-conversation"
        :label="t('messages.startup.label')"
        :description="t('messages.startup.help')"
        stacked
      >
        <div class="flex flex-wrap items-center gap-2">
          <AppSegmented
            :model-value="preferences.preferences.startup.mode"
            :label="t('messages.startup.label')"
            :options="[
              { value: 'last', label: t('messages.startup.last') },
              { value: 'channel', label: t('messages.startup.channel') },
            ]"
            @update:model-value="setStartupMode"
          />
          <AppSelect
            v-if="preferences.preferences.startup.mode === 'channel'"
            :model-value="startupChannel"
            :options="startupOptions"
            :placeholder="t('messages.startup.choose')"
            :aria-label="t('messages.startup.choose')"
            :disabled="!startupOptions.length"
            @update:model-value="setStartupChannel"
          />
        </div>
      </SettingRow>
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
