<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { administrationAccessQuery } from '../queries/administration'
import { awayLabel } from '../../shared/utils/commands'
import { speechConfigured } from '../../shared/speech-service'
// Search and Inbox live in the conversation list; search within a conversation is in its header.
const chat = useChatStore()
const actions = useWorkspaceActions()
const menus = useMenus()
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
    <div class="workspace-tools ml-auto flex shrink-0 items-center gap-1">
      <AppTooltip
        v-if="administration.data.value?.owner"
        :text="$t('workspaceUi.shell.administration')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
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
          class="aria-pressed:bg-selection aria-pressed:text-primary"
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
          :aria-label="$t('workspaceUi.shell.userSettings')"
          :aria-keyshortcuts="shortcutKeys.aria('settings')"
          @click="actions.settings()"
          ><AppIcon
            name="settings"
            :size="17" /></UiButton
      ></AppTooltip>
      <button
        class="self-profile ml-1 flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 text-left hover:bg-secondary [&_.presence]:border-rail [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
        :aria-label="$t('workspaceUi.shell.openProfile')"
        @click="actions.profile('you')"
        @contextmenu="chat.data && menus.member($event, chat.data.user)"
      >
        <UserAvatar
          :member="chat.self"
          size="small"
          presence
        /><span class="flex max-w-36 min-w-0 flex-col max-[700px]:hidden">
          <span class="truncate text-[length:var(--ui-text)] font-medium">{{
            chat.self?.displayName
          }}</span>
          <small
            v-if="chat.self?.status === 'away'"
            class="truncate text-muted-foreground"
            :title="chat.self.awayMessage"
            >{{ awayLabel(chat.self.awayMessage, $t('workspaceUi.presence.away')) }}</small
          >
        </span>
      </button>
    </div>
  </header>
</template>
