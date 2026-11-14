<script setup vapor lang="ts">
/**
 * What you see and who reaches you: sensitive media, third-party embeds, and ignored people.
 * Ignored people follow the account’s notification rules.
 */
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
const { t: $t } = useI18n()
/** Earlier per-server opt-ins count as on; switching off clears them too. */
const showNsfw = computed(
  () =>
    preferences.preferences.showNsfw ||
    Object.values(preferences.preferences.nsfwNetworks).some(Boolean),
)
function setNsfw(enabled: boolean) {
  preferences.update({ showNsfw: enabled, nsfwNetworks: {} })
}
</script>

<template>
  <div class="privacy-settings grid gap-6">
    <SettingGroup :title="$t('privacy.groups.media')">
      <SettingRow
        data-setting="nsfw-content"
        :label="$t('privacy.showNsfw')"
        :description="$t('privacy.showNsfwHelp')"
      >
        <AppSwitch
          :model-value="preferences.ready && showNsfw"
          :disabled="!preferences.ready"
          :aria-label="$t('privacy.showNsfw')"
          @update:model-value="setNsfw"
        />
      </SettingRow>
      <SettingRow
        data-setting="ask-before-embeds"
        :label="$t('mediaUi.messageSettings.askBeforeEmbeds')"
        :description="$t('mediaUi.messageSettings.askBeforeEmbedsHelp')"
      >
        <AppSwitch
          :model-value="preferences.preferences.askBeforeEmbeds"
          :aria-label="$t('mediaUi.messageSettings.askBeforeEmbeds')"
          @update:model-value="preferences.update({ askBeforeEmbeds: $event })"
        />
      </SettingRow>
    </SettingGroup>

    <SettingGroup :title="$t('messages.groups.hidden')">
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
  </div>
</template>
