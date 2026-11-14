<!-- Reka's as-child trigger needs VNode slots. -->
<script setup lang="ts">
import { PopoverAnchor } from 'reka-ui'
import { statusShowing } from '../../../shared/utils/profile'
import { awayLabel } from '../../../shared/utils/commands'

const chat = useChatStore()
const auth = useAuthStore()
const actions = useWorkspaceActions()
const menus = useMenus()
const { t } = useI18n()
const menuOpen = ref(false)
const editorOpen = ref(false)
const editorNetwork = ref('')
const pending = ref(false)
const skipReturnFocus = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const statusEditor = ref<{ suggestionsOpen: boolean } | null>(null)
let outsideEditor = false
const connected = computed(
  () => chat.status === 'connected' && chat.network?.status === 'connected',
)
const displayMember = computed(() =>
  chat.self && !connected.value ? { ...chat.self, status: 'offline' as const } : chat.self,
)
const presence = computed(() =>
  !connected.value
    ? t('selfMenu.offline')
    : chat.self?.status === 'away'
      ? t('workspaceUi.presence.away')
      : t('selfMenu.online'),
)
const hasStatus = computed(() => !!chat.self && statusShowing(chat.self))
const status = computed(() =>
  hasStatus.value ? [chat.self?.statusEmoji, chat.self?.statusText].filter(Boolean).join(' ') : '',
)
watch([() => chat.networkId, () => auth.account?.id], () => {
  menuOpen.value = false
  editorOpen.value = false
})
function openEditor() {
  const network = chat.networkId
  menuOpen.value = false
  if (!network) return
  skipReturnFocus.value = true
  if (!actions.status(network)) {
    outsideEditor = false
    editorNetwork.value = network
    editorOpen.value = true
  }
}
function run(action: () => void) {
  menuOpen.value = false
  action()
}
async function clearStatus() {
  const id = chat.networkId
  if (!id || pending.value) return
  pending.value = true
  try {
    await chat.networkProfile.mutateAsync({
      id,
      input: { statusText: '', statusEmoji: '', statusExpiresAt: '' },
    })
  } catch (e) {
    chat.notify(errorText(e, (key) => t(key)))
  } finally {
    pending.value = false
  }
}
async function toggleAway() {
  const id = chat.networkId
  if (!id || !connected.value || pending.value) return
  pending.value = true
  try {
    await apiClient().updateSelf(id, { away: chat.self?.status === 'away' ? null : 'Away' })
  } catch (e) {
    chat.notify(errorText(e, (key) => t(key)))
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <UiPopover
    v-model:open="editorOpen"
    :modal="false"
  >
    <PopoverAnchor as-child>
      <span class="inline-flex">
        <UiDropdownMenu
          v-model:open="menuOpen"
          :modal="false"
        >
          <UiDropdownMenuTrigger as-child>
            <button
              ref="trigger"
              type="button"
              data-self-menu-trigger
              class="self-profile flex min-h-8 max-w-[190px] min-w-0 items-center gap-2 rounded-full bg-secondary/55 py-0.5 pr-3 pl-1.5 text-left text-[length:var(--ui-small)] font-medium text-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_65%,transparent)] transition-colors hover:bg-accent max-[700px]:px-1.5 [&_.presence]:border-sidebar hover:[&_.presence]:border-accent [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
              :aria-label="t('selfMenu.open')"
              @contextmenu="chat.self && menus.member($event, chat.self)"
            >
              <UserAvatar
                :member="displayMember"
                size="small"
                presence
              />
              <span class="flex max-w-36 min-w-0 flex-col justify-center max-[700px]:hidden">
                <span class="truncate text-[length:var(--ui-text)] font-medium leading-[1.1]">{{
                  chat.self?.displayName
                }}</span>
                <small
                  v-if="status || chat.self?.status === 'away'"
                  class="flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap text-[length:var(--ui-caption)] leading-[1.1] text-muted-foreground"
                >
                  <template v-if="hasStatus">
                    <span
                      v-if="chat.self?.statusEmoji"
                      class="inline-flex h-3.5 max-w-8 shrink-0 items-center overflow-hidden text-[14px] leading-none [&_.icon-custom-emoji]:size-3.5"
                    >
                      <StatusEmoji :value="chat.self.statusEmoji" />
                    </span>
                    <span class="min-w-0 truncate">{{ chat.self?.statusText }}</span>
                  </template>
                  <span
                    v-else
                    class="min-w-0 truncate"
                    >{{ awayLabel(chat.self?.awayMessage, t('workspaceUi.presence.away')) }}</span
                  >
                </small>
              </span>
            </button>
          </UiDropdownMenuTrigger>
          <UiDropdownMenuContent
            align="end"
            class="w-72 max-w-[calc(100vw-16px)]"
            :aria-label="t('selfMenu.open')"
            @close-auto-focus="
              (event) => {
                if (skipReturnFocus) {
                  event.preventDefault()
                  skipReturnFocus = false
                }
              }
            "
          >
            <div class="flex items-center gap-2.5 px-2 py-2">
              <UserAvatar
                :member="displayMember"
                size="normal"
                presence
              />
              <span class="grid min-w-0 gap-0.5">
                <strong class="truncate">{{ chat.self?.displayName || t('selfMenu.you') }}</strong>
                <span class="truncate text-[length:var(--ui-small)] text-muted-foreground"
                  >{{ chat.network?.name || t('selfMenu.noNetwork') }} · {{ presence }}</span
                >
              </span>
            </div>
            <UiDropdownMenuItem
              :disabled="!chat.networkId"
              class="my-1 rounded-md bg-secondary/55"
              @select="openEditor"
            >
              <AppIcon
                name="smile"
                :size="16"
              /><span class="min-w-0 flex-1 truncate"
                ><template v-if="hasStatus"
                  ><StatusEmoji
                    v-if="chat.self?.statusEmoji"
                    :value="chat.self.statusEmoji"
                  />
                  {{ chat.self?.statusText }}</template
                ><template v-else>{{ t('selfMenu.setStatus') }}</template></span
              >
            </UiDropdownMenuItem>
            <UiDropdownMenuItem
              v-if="hasStatus"
              :disabled="pending"
              @select="clearStatus"
              >{{ t('selfMenu.clearStatus') }}</UiDropdownMenuItem
            >
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem @select="run(() => actions.profile('you'))">{{
              t('selfMenu.viewProfile')
            }}</UiDropdownMenuItem>
            <UiDropdownMenuItem @select="run(() => actions.settings('profile'))">{{
              t('selfMenu.profileSettings')
            }}</UiDropdownMenuItem>
            <UiDropdownMenuItem
              :disabled="!connected || pending"
              @select="toggleAway"
              >{{
                chat.self?.status === 'away' ? t('selfMenu.setBack') : t('selfMenu.setAway')
              }}</UiDropdownMenuItem
            >
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem @select="run(() => actions.settings())">{{
              t('selfMenu.preferences')
            }}</UiDropdownMenuItem>
            <UiDropdownMenuItem @select="run(() => actions.settings('notifications'))">{{
              t('selfMenu.notifications')
            }}</UiDropdownMenuItem>
            <UiDropdownMenuItem @select="run(() => actions.settings('account'))">{{
              t('selfMenu.accountSettings')
            }}</UiDropdownMenuItem>
            <UiDropdownMenuSeparator />
            <UiDropdownMenuItem @select="run(() => void auth.confirmLogout())">{{
              t('common.logOut')
            }}</UiDropdownMenuItem>
          </UiDropdownMenuContent>
        </UiDropdownMenu>
      </span>
    </PopoverAnchor>
    <UiPopoverContent
      align="end"
      class="w-[min(28rem,calc(100vw-16px))] p-0"
      @open-auto-focus.prevent
      @pointer-down-outside="outsideEditor = true"
      @focus-outside="outsideEditor = true"
      @escape-key-down="
        (event) => {
          if (statusEditor?.suggestionsOpen) event.preventDefault()
        }
      "
      @close-auto-focus="
        (event) => {
          if (!outsideEditor) {
            event.preventDefault()
            trigger?.focus()
          }
          outsideEditor = false
        }
      "
    >
      <StatusEditor
        v-if="editorOpen"
        ref="statusEditor"
        :network-id="editorNetwork"
        @close="editorOpen = false"
      />
    </UiPopoverContent>
  </UiPopover>
</template>
