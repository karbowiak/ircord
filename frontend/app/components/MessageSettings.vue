<script setup lang="ts">
/**
 * Settings → Messages & media: how conversations look and what plays in them. Layout values
 * belong to the appearance (synced with “Same look on every device”); activity, previews and
 * autoplay are account preferences; ignored people are part of the account's notification
 * rules, so ignoring someone also keeps them from notifying you.
 */
import {
  appearanceDefaults,
  messageAppearanceKeys,
  messageFonts,
  ranges,
  type Appearance,
} from '../../shared/appearance'
import { scopeLabels } from '../../shared/settings-index'
const chat = useChatStore()
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

const ignore = ref('')
function addIgnore() {
  const value = ignore.value.trim()
  if (value) notifications.update({ ignore: [...notifications.settings.ignore, value] })
  ignore.value = ''
}
</script>

<template>
  <div class="appearance-settings">
    <MessagePreview />

    <SettingGroup title="Layout">
      <template #scope
        >{{ layoutScope }} ·
        <SettingLink to="appearance">Change in Appearance</SettingLink></template
      >
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
      <SettingRow
        data-setting="message-font"
        label="Message font"
        stacked
      >
        <div
          data-setting="text-size"
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
              >{{ messageFontSample }}</span
            >
            <strong>{{ font.label }}</strong>
            <small>{{ font.note }}</small>
          </button>
        </div>
      </SettingRow>
      <SettingRow label="Text size">
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
      </SettingRow>
      <SettingRow
        data-setting="group-spacing"
        label="Space between groups"
        description="Gap before each new speaker."
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

    <SettingGroup title="Media and animation">
      <p class="setting-note">
        <AppIcon
          name="info"
          :size="15"
        /><span
          >Autoplay covers GIFs and videos in messages and syncs to your account. Animated emoji
          covers custom emoji everywhere and GIFs in the pickers, and follows your appearance sync.
          <SettingLink to="appearance">Reduce motion</SettingLink> overrides both: while it’s on,
          nothing plays by itself.</span
        >
      </p>
      <SettingRow
        data-setting="link-previews"
        label="Image and video previews"
        :description="
          chat.data?.capabilities.mediaCDN
            ? 'Stored images and videos load from IRCord. Trusted includes these copies from other public hosts; All also previews other supported link types. Provider players load when you press Play.'
            : 'Loading a preview shows the linked site your IP address. “Trusted” previews only well-known image hosts, YouTube thumbnails and hosts a channel’s operators add for that channel; YouTube players load only when you press play.'
        "
      >
        <AppSegmented
          :model-value="preferences.preferences.linkPreviews"
          :options="[
            { value: 'off', label: 'Off' },
            { value: 'trusted', label: 'Trusted' },
            { value: 'all', label: 'All' },
          ]"
          label="Link previews"
          @update:model-value="preferences.update({ linkPreviews: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="autoplay-media"
        label="Autoplay GIFs and videos"
        :description="
          appearance.reducedMotion
            ? 'Reduce motion is on, so GIFs and videos in messages stay still. Open one to play it.'
            : 'In messages. When off, they play while you hover or focus them. On touch screens, tap a GIF once to play it; videos use their play button.'
        "
      >
        <AppSwitch
          :model-value="preferences.preferences.autoplayMedia"
          aria-label="Autoplay GIFs and videos"
          @change="preferences.update({ autoplayMedia: $event })"
        />
      </SettingRow>
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
        <form
          class="chip-input"
          @submit.prevent="addIgnore"
        >
          <span
            v-for="mask in notifications.settings.ignore"
            :key="mask"
            class="chip"
            >{{ mask
            }}<button
              type="button"
              :aria-label="`Stop ignoring ${mask}`"
              @click="
                notifications.update({
                  ignore: notifications.settings.ignore.filter((x) => x !== mask),
                })
              "
            >
              <AppIcon
                name="close"
                :size="12"
              /></button
          ></span>
          <input
            v-model="ignore"
            aria-label="Ignore a nick or mask"
            placeholder="Add a nick or mask, press Enter"
            maxlength="120"
          />
        </form>
      </SettingRow>
    </SettingGroup>

    <div
      data-setting="messages-reset"
      class="settings-footer"
    >
      <button
        class="button secondary small"
        @click="update(appearanceDefaults(messageAppearanceKeys))"
      >
        <AppIcon
          name="reset"
          :size="14"
        />Reset message layout
      </button>
    </div>
  </div>
</template>
