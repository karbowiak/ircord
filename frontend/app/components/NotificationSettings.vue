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
const { t } = useI18n()
const levelOptions = computed(
  () =>
    [
      { value: 'all', label: t('notifications.levels.all') },
      { value: 'mentions', label: t('notifications.levels.mentions') },
      { value: 'none', label: t('notifications.levels.nothing') },
    ] as const,
)
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <NotificationDevice />

    <NotificationRouting />

    <SettingGroup
      :title="$t('notifications.groups.quietHours')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingRow
        data-setting="quiet-hours"
        :label="$t('notifications.quietHours.silence')"
        :description="
          $t('notifications.quietHours.silenceHelp', {
            zone: notifications.settings.quietHours.timeZone,
          })
        "
      >
        <AppSwitch
          :model-value="notifications.settings.quietHours.enabled"
          :aria-label="$t('notifications.groups.quietHours')"
          @update:model-value="setQuiet({ enabled: $event })"
        />
      </SettingRow>
      <div
        v-if="notifications.settings.quietHours.enabled"
        data-setting="quiet-hours-times"
        class="setting-row quiet-times flex items-center justify-start gap-4 [&_[data-slot=input]]:w-30 [&_label]:flex [&_label]:items-center [&_label]:gap-2 [&_label]:text-[13px] [&_label]:text-muted-foreground"
      >
        <label
          >{{ $t('notifications.quietHours.from') }}
          <UiInput
            type="time"
            :model-value="notifications.settings.quietHours.start"
            @change="setQuiet({ start: ($event.target as HTMLInputElement).value })" /></label
        ><label
          >{{ $t('notifications.quietHours.until') }}
          <UiInput
            type="time"
            :model-value="notifications.settings.quietHours.end"
            @change="setQuiet({ end: ($event.target as HTMLInputElement).value })"
        /></label>
      </div>
    </SettingGroup>

    <SettingGroup
      :title="$t('notifications.groups.notifyAbout')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingRow
        data-setting="channel-notifications"
        :label="$t('notifications.channels')"
        :description="$t('notifications.channelsHelp')"
      >
        <AppSegmented
          :model-value="notifications.settings.channelDefault"
          :options="levelOptions"
          :label="$t('notifications.channelsAria')"
          @update:model-value="notifications.update({ channelDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="direct-notifications"
        :label="$t('notifications.directMessages')"
      >
        <AppSegmented
          :model-value="notifications.settings.directDefault"
          :options="levelOptions"
          :label="$t('notifications.directAria')"
          @update:model-value="notifications.update({ directDefault: $event })"
        />
      </SettingRow>
      <SettingRow
        data-setting="highlight-words"
        :label="$t('notifications.highlightWords')"
        :description="$t('notifications.highlightWordsHelp')"
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
          :remove-label="(value) => $t('notifications.removeHighlight', { value })"
          :aria-label="$t('notifications.highlightAria')"
          :placeholder="$t('notifications.highlightPlaceholder')"
          maxlength="64"
        />
      </SettingRow>
      <SettingNote as="p">
        <AppIcon
          name="info"
          :size="15"
        /><span>
          <i18n-t keypath="notifications.ignoredNote">
            <template #link><SettingLink to="messages" /></template>
          </i18n-t>
        </span>
      </SettingNote>
    </SettingGroup>
  </div>
</template>
