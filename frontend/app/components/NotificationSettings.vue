<script setup lang="ts">
import type { DeviceMode } from '../stores/notifications'
const notifications = useNotificationStore()
const sound = useAlertSound()
/** Quiet hours follow this device's time zone whenever they're edited here. */
function setQuiet(changes: Partial<{ enabled: boolean; start: string; end: string }>) {
  notifications.update({
    quietHours: {
      ...notifications.settings.quietHours,
      ...changes,
      timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    },
  })
}
const preferences = usePreferencesStore()
const appearance = useAppearanceStore()
const install = useInstallPrompt()
const word = ref('')
const ignore = ref('')
const levelOptions = [
  { value: 'all', label: 'All' },
  { value: 'mentions', label: 'Mentions' },
  { value: 'none', label: 'Nothing' },
] as const
const modes: { value: DeviceMode; label: string; note: string }[] = [
  { value: 'off', label: 'Off', note: 'No notifications on this device.' },
  { value: 'open', label: 'While open', note: 'Only while IRCord is open in a tab or window.' },
  { value: 'push', label: 'Always', note: 'Even when IRCord is closed, through push.' },
]
const blocked = computed(() => notifications.permission === 'denied')
function addWord() {
  const value = word.value.trim()
  if (value)
    notifications.update({ highlightWords: [...notifications.settings.highlightWords, value] })
  word.value = ''
}
function addIgnore() {
  const value = ignore.value.trim()
  if (value) notifications.update({ ignore: [...notifications.settings.ignore, value] })
  ignore.value = ''
}
</script>
<template>
  <div class="appearance-settings">
    <section class="setting-group">
      <h3>This device</h3>
      <div
        v-if="notifications.needsInstall"
        class="install-hint"
      >
        <AppIcon
          name="download"
          :size="18"
        />
        <span class="setting-copy"
          ><strong>Add IRCord to your Home Screen first</strong
          ><small
            >On iPhone and iPad, notifications only work in the installed app: tap Share, then “Add
            to Home Screen”, and open IRCord from there.</small
          ></span
        >
      </div>
      <div
        v-else-if="install.available"
        class="install-hint"
      >
        <AppIcon
          name="download"
          :size="18"
        />
        <span class="setting-copy"
          ><strong>Install IRCord as an app</strong
          ><small
            >Its own window, an unread badge on the icon and more reliable notifications.</small
          ></span
        >
        <button
          class="button secondary small"
          @click="install.prompt()"
        >
          Install
        </button>
      </div>
      <div class="setting-row stacked">
        <span class="setting-copy"
          ><strong>Notifications</strong
          ><small>{{
            blocked
              ? 'Blocked by your browser. Allow notifications for this site in its settings.'
              : !notifications.supported.notifications
                ? 'This browser can’t show notifications.'
                : modes.find((m) => m.value === notifications.mode)?.note
          }}</small></span
        >
        <div
          class="segmented"
          role="radiogroup"
          aria-label="Notifications on this device"
        >
          <button
            v-for="m in modes"
            :key="m.value"
            role="radio"
            :aria-checked="notifications.mode === m.value"
            :class="{ active: notifications.mode === m.value }"
            :disabled="
              notifications.busy ||
              (m.value !== 'off' && (blocked || !notifications.supported.notifications)) ||
              (m.value === 'push' && (!notifications.supported.push || notifications.needsInstall))
            "
            @click="notifications.setMode(m.value)"
          >
            {{ m.label }}
          </button>
        </div>
      </div>
      <div
        v-if="notifications.mode === 'push'"
        class="setting-row"
      >
        <span class="setting-copy"
          ><strong>Test</strong
          ><small>Sends a notification to every device using push.</small></span
        >
        <button
          class="button secondary small"
          @click="notifications.test()"
        >
          Send test
        </button>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Sound</strong
          ><small
            >A short chime while IRCord is open, unless you’re looking at that conversation.</small
          ></span
        >
        <div class="param-controls">
          <div
            class="segmented"
            role="radiogroup"
            aria-label="Notification sound"
          >
            <button
              v-for="option in [
                { value: 'off', label: 'Off' },
                { value: 'mentions', label: 'Mentions' },
                { value: 'all', label: 'All' },
              ] as const"
              :key="option.value"
              role="radio"
              :aria-checked="sound.mode.value === option.value"
              :class="{ active: sound.mode.value === option.value }"
              @click="sound.setMode(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
          <button
            class="icon-button"
            aria-label="Play the sound"
            title="Preview"
            :disabled="sound.mode.value === 'off'"
            @click="sound.preview()"
          >
            <AppIcon
              name="volume"
              :size="16"
            />
          </button>
        </div>
      </div>
    </section>

    <section class="setting-group">
      <h3>Quiet hours</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Silence notifications</strong
          ><small
            >No push, pop-ups or sounds between these times ({{
              notifications.settings.quietHours.timeZone
            }}).</small
          ></span
        >
        <button
          role="switch"
          class="toggle"
          :class="{ on: notifications.settings.quietHours.enabled }"
          :aria-checked="notifications.settings.quietHours.enabled"
          aria-label="Quiet hours"
          @click="setQuiet({ enabled: !notifications.settings.quietHours.enabled })"
        >
          <span />
        </button>
      </div>
      <div
        v-if="notifications.settings.quietHours.enabled"
        class="setting-row quiet-times"
      >
        <label
          >From
          <input
            type="time"
            class="text-input"
            :value="notifications.settings.quietHours.start"
            @change="setQuiet({ start: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >until
          <input
            type="time"
            class="text-input"
            :value="notifications.settings.quietHours.end"
            @change="setQuiet({ end: ($event.target as HTMLInputElement).value })"
        /></label>
      </div>
    </section>

    <section class="setting-group">
      <h3>Notify me about</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Channels</strong
          ><small>Change a single channel from its right-click menu.</small></span
        >
        <div
          class="segmented"
          role="group"
          aria-label="Channel notifications"
        >
          <button
            v-for="option in levelOptions"
            :key="option.value"
            :class="{ active: notifications.settings.channelDefault === option.value }"
            :aria-pressed="notifications.settings.channelDefault === option.value"
            @click="notifications.update({ channelDefault: option.value })"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
      <div class="setting-row">
        <span class="setting-copy"><strong>Direct messages</strong></span>
        <div
          class="segmented"
          role="group"
          aria-label="Direct message notifications"
        >
          <button
            v-for="option in levelOptions"
            :key="option.value"
            :class="{ active: notifications.settings.directDefault === option.value }"
            :aria-pressed="notifications.settings.directDefault === option.value"
            @click="notifications.update({ directDefault: option.value })"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
      <div class="setting-row stacked">
        <span class="setting-copy"
          ><strong>Highlight words</strong
          ><small>Count as a mention, like your nick. Whole words, any case.</small></span
        >
        <form
          class="chip-input"
          @submit.prevent="addWord"
        >
          <span
            v-for="w in notifications.settings.highlightWords"
            :key="w"
            class="chip"
            >{{ w
            }}<button
              type="button"
              :aria-label="`Remove ${w}`"
              @click="
                notifications.update({
                  highlightWords: notifications.settings.highlightWords.filter((x) => x !== w),
                })
              "
            >
              <AppIcon
                name="close"
                :size="12"
              /></button
          ></span>
          <input
            v-model="word"
            aria-label="Add a highlight word"
            placeholder="Add a word, press Enter"
            maxlength="64"
          />
        </form>
      </div>
      <div class="setting-row stacked">
        <span class="setting-copy"
          ><strong>Ignored people</strong
          ><small
            >Their messages are hidden and never notify. A nick, or a mask like
            <code>*!*@host.example</code>.</small
          ></span
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
      </div>
    </section>

    <section class="setting-group">
      <h3>Conversation</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Channel activity</strong
          ><small
            >Small activity lines. Kicks, bans and topic changes remain visible; duplicate history
            notices go to Server.</small
          ></span
        >
        <div
          class="segmented"
          role="group"
          aria-label="Join and part messages"
        >
          <button
            :class="{ active: preferences.preferences.events !== 'hide' }"
            :aria-pressed="preferences.preferences.events !== 'hide'"
            @click="preferences.update({ events: 'collapse' })"
          >
            Compact
          </button>
          <button
            :class="{ active: preferences.preferences.events === 'hide' }"
            :aria-pressed="preferences.preferences.events === 'hide'"
            @click="preferences.update({ events: 'hide' })"
          >
            Hide
          </button>
        </div>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Image and video previews</strong
          ><small
            >Loading a preview shows the linked site your IP address. “Trusted” previews only
            well-known image hosts.</small
          ></span
        >
        <div
          class="segmented"
          role="group"
          aria-label="Link previews"
        >
          <button
            v-for="option in [
              { value: 'off', label: 'Off' },
              { value: 'trusted', label: 'Trusted' },
              { value: 'all', label: 'All' },
            ] as const"
            :key="option.value"
            :class="{ active: preferences.preferences.linkPreviews === option.value }"
            :aria-pressed="preferences.preferences.linkPreviews === option.value"
            @click="preferences.update({ linkPreviews: option.value })"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Autoplay GIFs and videos</strong
          ><small>{{
            appearance.reducedMotion
              ? 'Reduced motion is on, so GIFs and videos stay still. Open one to play it.'
              : 'When off, they play while you hover or focus them. On touch screens, tap a GIF once to play it; videos use their play button.'
          }}</small></span
        >
        <button
          role="switch"
          class="toggle"
          :class="{ on: preferences.preferences.autoplayMedia }"
          :aria-checked="preferences.preferences.autoplayMedia"
          aria-label="Autoplay GIFs and videos"
          @click="preferences.update({ autoplayMedia: !preferences.preferences.autoplayMedia })"
        >
          <span />
        </button>
      </div>
    </section>
  </div>
</template>
