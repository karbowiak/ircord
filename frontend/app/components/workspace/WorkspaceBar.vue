<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import { administrationAccessQuery } from '../../queries/administration'
import { speechConfigured } from '../../../shared/speech-service'
const { t: $t } = useI18n()
// Search and Inbox live in the conversation list; search within a conversation is in its header.
const actions = useWorkspaceActions()
const shortcutKeys = useShortcuts()
const administration = useQuery(administrationAccessQuery)
const readAloud = useReadAloud()
const preferences = usePreferencesStore()
/** The read-aloud switch appears once a speech service is set up in Settings. */
const speechReady = computed(() => speechConfigured(preferences.preferences.speech))
const readingAloud = computed(() => readAloud.enabled.value && speechReady.value)
</script>
<template>
  <header class="workspace-bar flex min-w-0 items-center gap-2 pr-2">
    <NetworkRail @add="actions.addNetwork()" />
    <div class="workspace-tools ml-auto flex shrink-0 items-center gap-1.5 pr-1">
      <AppTooltip
        v-if="administration.data.value?.owner"
        :text="$t('workspaceUi.shell.administration')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent"
          :aria-label="$t('workspaceUi.shell.administration')"
          @click="actions.administration()"
          ><AppIcon
            name="shield"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip
        v-if="speechReady"
        :text="
          readingAloud ? $t('workspaceUi.shell.stopReading') : $t('workspaceUi.shell.startReading')
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent aria-pressed:bg-selection aria-pressed:text-primary"
          :aria-label="$t('workspaceUi.shell.readAloud')"
          :aria-pressed="readingAloud"
          @click="readAloud.setEnabled(!readingAloud)"
          ><AppIcon
            name="read-aloud"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip :text="shortcutKeys.hinted($t('workspaceUi.shell.settings'), 'settings')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent min-[701px]:h-8 min-[701px]:w-auto min-[701px]:gap-1.5 min-[701px]:px-3"
          :aria-label="$t('workspaceUi.shell.userSettings')"
          :aria-keyshortcuts="shortcutKeys.aria('settings')"
          @click="actions.settings()"
          ><AppIcon
            name="settings"
            :size="16"
          /><span
            class="text-[length:var(--ui-small)] font-medium text-foreground max-[700px]:hidden"
            >{{ $t('workspaceUi.shell.settings') }}</span
          ></UiButton
        ></AppTooltip
      >
      <SelfMenu />
    </div>
  </header>
</template>
