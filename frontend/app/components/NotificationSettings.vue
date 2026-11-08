<script setup lang="ts">
/**
 * Settings → Notifications: when, where and how IRCord alerts you. This device and routing across
 * devices are their own components; quiet hours and what notifies you stay here.
 */
import { scopeLabels } from '../../shared/settings-index'
const notifications = useNotificationStore()
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
const levelOptions = [
  { value: 'all', label: 'All' },
  { value: 'mentions', label: 'Mentions' },
  { value: 'none', label: 'Nothing' },
] as const
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <NotificationDevice />

    <NotificationRouting />

    <SettingGroup
      title="Quiet hours"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="quiet-hours"
        label="Silence notifications"
        :description="`No push, pop-ups or sounds between these times (${notifications.settings.quietHours.timeZone}).`"
      >
        <AppSwitch
          :model-value="notifications.settings.quietHours.enabled"
          aria-label="Quiet hours"
          @update:model-value="setQuiet({ enabled: $event })"
        />
      </SettingRow>
      <div
        v-if="notifications.settings.quietHours.enabled"
        data-setting="quiet-hours-times"
        class="setting-row quiet-times flex items-center justify-start gap-4"
      >
        <label
          >From
          <UiInput
            type="time"
            :model-value="notifications.settings.quietHours.start"
            @change="setQuiet({ start: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >until
          <UiInput
            type="time"
            :model-value="notifications.settings.quietHours.end"
            @change="setQuiet({ end: ($event.target as HTMLInputElement).value })"
        /></label>
      </div>
    </SettingGroup>

    <SettingGroup
      title="Notify me about"
      :scope="scopeLabels.account"
    >
      <SettingRow
        data-setting="channel-notifications"
        label="Channels"
        description="Used where a channel’s level is Default. Each channel’s level is set by its operators in channel settings."
      >
        <AppSegmented
          :model-value="notifications.settings.channelDefault"
          :options="levelOptions"
          label="Channel notifications"
          @update:model-value="notifications.update({ channelDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="direct-notifications"
        label="Direct messages"
      >
        <AppSegmented
          :model-value="notifications.settings.directDefault"
          :options="levelOptions"
          label="Direct message notifications"
          @update:model-value="notifications.update({ directDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="highlight-words"
        label="Highlight words"
        description="Count as a mention, like your nick. Whole words, any case."
        stacked
      >
        <ChipInput
          :items="notifications.settings.highlightWords"
          :add="
            (value) =>
              notifications.update({
                highlightWords: [...notifications.settings.highlightWords, value],
              })
          "
          :remove="
            (value) =>
              notifications.update({
                highlightWords: notifications.settings.highlightWords.filter((x) => x !== value),
              })
          "
          :remove-label="(value) => `Remove ${value}`"
          aria-label="Add a highlight word"
          placeholder="Add a word, press Enter"
          maxlength="64"
        />
      </SettingRow>
      <SettingNote as="p">
        <AppIcon
          name="info"
          :size="15"
        /><span
          >People you ignore never notify you. Manage them under
          <SettingLink to="messages" />.</span
        >
      </SettingNote>
    </SettingGroup>
  </div>
</template>
