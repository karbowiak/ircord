<script setup lang="ts" vapor>
import { useQuery } from '@pinia/colada'
import { administrationAccessQuery } from '../../queries/administration'
import { awayLabel } from '../../../shared/utils/commands'
import { speechConfigured } from '../../../shared/speech-service'
const { t: $t } = useI18n()
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
      <button
        class="self-profile flex h-8 max-w-[190px] min-w-0 items-center gap-2 rounded-full bg-secondary/55 py-0 pr-3 pl-1.5 max-[700px]:px-1.5 text-left text-[length:var(--ui-small)] font-medium text-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_65%,transparent)] transition-colors hover:bg-accent [&_.presence]:border-sidebar hover:[&_.presence]:border-accent [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
        :aria-label="
          $t('workspaceUi.shell.openProfile', {
            name:
              chat.self?.status === 'away'
                ? `${chat.self.displayName} ${awayLabel(chat.self.awayMessage, $t('workspaceUi.presence.away'))}`
                : chat.self?.displayName,
          })
        "
        @click="actions.profile('you')"
        @contextmenu="chat.self && menus.member($event, chat.self)"
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
