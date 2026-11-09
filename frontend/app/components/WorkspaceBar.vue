<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { administrationAccessQuery } from '../queries/administration'
import { ariaShortcut, isMac, shortcutLabel } from '../../shared/utils/shortcuts'
import { speechConfigured } from '../../shared/speech-service'
// Search and Inbox live in the conversation list; search within a conversation is in its header.
const chat = useChatStore()
const actions = useWorkspaceActions()
const menus = useMenus()
const mac = isMac(navigator.platform)
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
        text="Administration"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          aria-label="Administration"
          @click="actions.administration()"
          ><AppIcon
            name="shield"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip
        v-if="speechReady"
        :text="
          readingAloud ? 'Stop reading highlights aloud' : 'Read highlights aloud on this device'
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="aria-pressed:bg-selection aria-pressed:text-primary"
          aria-label="Read highlights aloud"
          :aria-pressed="readingAloud"
          @click="readAloud.setEnabled(!readingAloud)"
          ><AppIcon
            name="read-aloud"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip :text="`Settings (${shortcutLabel('settings', mac)})`"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          aria-label="User settings"
          :aria-keyshortcuts="ariaShortcut('settings', mac)"
          @click="actions.settings()"
          ><AppIcon
            name="settings"
            :size="17" /></UiButton
      ></AppTooltip>
      <button
        class="self-profile ml-1 flex min-w-0 items-center gap-2 rounded-md px-1.5 py-1 text-left hover:bg-secondary [&_.presence]:border-rail [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
        aria-label="Open your profile"
        @click="actions.profile('you')"
        @contextmenu="chat.data && menus.member($event, chat.data.user)"
      >
        <UserAvatar
          :member="chat.self"
          size="small"
          presence
        /><span
          class="max-w-28 truncate text-[length:var(--ui-text)] font-medium max-[700px]:hidden"
          >{{ chat.self?.displayName }}</span
        >
      </button>
    </div>
  </header>
</template>
