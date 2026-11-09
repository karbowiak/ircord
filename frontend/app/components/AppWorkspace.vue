<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import { useEventListener, useMediaQuery } from '@vueuse/core'
import { FocusScope } from 'reka-ui'
import type { Message, NetworkInput } from '../../shared/types/chat'
import { isConversationPath, workspacePages } from '../../shared/utils/workspace-routes'
import { messagesQuery } from '../queries/chat'
import { layoutLimits } from '../stores/layout'

/**
 * The signed-in app: layout (rail, sidebars, conversation), drawers on small screens, and the
 * overlays everything else opens through useWorkspaceActions().
 */
const chat = useChatStore()
const router = useRouter()
const route = useRoute()
const navigation = useConversationNavigation()
const layout = useLayoutStore()
const auth = useAuthStore()
// Open the archive before child history and search queries can request it.
useOfflineSnapshot()
const dialogs = useDialogStack()
const connectionGuideEnabled = useRuntimeConfig().public.firstConnectionGuide
usePreferencesStore()
// Which messages are saved, for every message's toolbar and menu.
useSavedStore()
// The inbox's unread indicator in the header.
useInboxStore()
// Toasts announce through the persistent announcer (errors interrupt), every time one is shown.
const toastAnnouncer = useStatusAnnouncer()
watch(
  () => chat.notice,
  (text) => {
    if (text) toastAnnouncer.say(text, chat.noticeError ? 'assertive' : 'polite')
    // A toast removed while hovered never sees pointerleave; the next one must still time out.
    else chat.holdNotice(false)
  },
)
const dismissNotice = () => (chat.notice = '')
// The session ended elsewhere (signed out, expired, account deleted): back to the login screen.
watch(
  () => chat.bootstrap.error,
  (error) => {
    if ((error as { statusCode?: number } | null)?.statusCode === 401) {
      chat.dispose()
      auth.expire()
    }
  },
)

// ---- Overlays ---------------------------------------------------------------------------------

type Overlay =
  'settings' | 'network' | 'join' | 'profile' | 'channel' | 'shortcuts' | 'administration'
const overlay = ref<Overlay | null>(null)
const settingsTab = ref('appearance')
/** Network being edited ('' adds one), and a prefill for new ones (irc:// links). */
const networkDialog = ref<{
  id: string
  prefill?: Partial<NetworkInput>
  firstConnection?: boolean
}>({ id: '' })
const setupKey = `ircord.connection-setup.${auth.account?.id ?? 'signed-out'}`
function closeNetworkDialog() {
  if (networkDialog.value.firstConnection) writeLocal(setupKey, true)
  overlay.value = null
}
/** Channel whose settings panel is open, and the tab it opened on. */
const channelPanel = ref({ id: '', tab: 'overview' })
const profileId = ref('you')
const replyTo = ref<Message>()
// The reply target is a snapshot; drop it once the message is deleted (by you on
// another device, by its author or by an operator) so the composer never quotes it.
const activeHistory = useQuery(() => messagesQuery(chat.activeId))
watch(
  () =>
    replyTo.value &&
    activeHistory.data.value?.items.find((m) => m.id === replyTo.value?.id)?.deleted,
  (deleted) => {
    if (!deleted) return
    replyTo.value = undefined
    chat.notify('The message you were replying to was deleted.')
  },
)
const highlightId = ref('')
let highlightTimer: ReturnType<typeof setTimeout> | undefined

function highlight(messageId: string) {
  overlay.value = null
  highlightId.value = messageId
  clearTimeout(highlightTimer)
  highlightTimer = setTimeout(() => (highlightId.value = ''), 3500)
}
const composer = ref<{ resume: () => boolean } | null>(null)
/** Opens a draft's conversation and puts the caret at the end of its text; nothing is sent. */
async function resumeDraft(conversationId: string) {
  await navigation.open(conversationId)
  if (chat.activeId !== conversationId) return
  overlay.value = null
  closeDrawers()
  // After the dialog has closed and handed focus back.
  // A disabled composer (offline, disconnected) can't take focus: the conversation can.
  nextTick(() =>
    nextTick(() => {
      if (!composer.value?.resume())
        document.querySelector<HTMLElement>('.timeline-scroll')?.focus({ preventScroll: true })
    }),
  )
}
const historyTools = useHistoryTools()
/** Pages reveal the source conversation without losing the draft being left. */
async function reveal(conversationId: string, messageId: string) {
  overlay.value = null
  closeDrawers()
  await navigation.open(conversationId)
  if (await historyTools.reveal(conversationId, messageId)) highlight(messageId)
}
const actions = useWorkspaceActions()
/** Confirmations and prompts, which the notification setup offer must not stack over. */
const prompt = usePrompt()
actions.profile = (id) => {
  profileId.value = id
  overlay.value = 'profile'
}
actions.settings = (tab = 'appearance') => {
  settingsTab.value = tab
  overlay.value = 'settings'
}
actions.editNetwork = (id = '') => {
  networkDialog.value = { id }
  overlay.value = 'network'
}
actions.addNetwork = (prefill) => {
  networkDialog.value = {
    id: '',
    prefill,
    firstConnection: !prefill && !chat.data?.demo && !chat.data?.networks.length,
  }
  overlay.value = 'network'
}
actions.channelSettings = (id, tab = 'overview') => {
  channelPanel.value = { id, tab }
  overlay.value = 'channel'
}
actions.join = () => (overlay.value = 'join')
async function openPage(path: string, query?: Record<string, string>) {
  overlay.value = null
  closeDrawers()
  await router.push({ path, query })
  if (path === '/search')
    nextTick(() => document.querySelector<HTMLInputElement>('.workspace-page [autofocus]')?.focus())
}
actions.search = (query) => {
  void openPage('/search', query ? { q: query } : undefined)
}
actions.drafts = () => {
  void openPage('/drafts')
}
actions.saved = () => {
  void openPage('/saved')
}
actions.links = (conversationId = '') => {
  void openPage('/links', conversationId ? { channel: conversationId } : undefined)
}
actions.inbox = () => {
  void openPage('/inbox')
}
actions.reveal = reveal
actions.resumeDraft = resumeDraft
actions.shortcuts = () => (overlay.value = 'shortcuts')
actions.administration = () => (overlay.value = 'administration')
function requestReply(message: Message) {
  const reason = chat.messageActionReason(message, 'replies')
  if (reason) return chat.notify(reason)
  replyTo.value = message
}
actions.reply = requestReply
actions.exportLog = (id) => void historyTools.exportLog(id)
actions.jumpToDate = (id) => void historyTools.jumpToDate(id, highlight)

// Only the initial empty workspace opens the guide automatically. Deep links
// retain their explicit connection setup, and dismissing the guide is remembered
// for this account on this device. Add Network always lets the user return.
let checkedFirstConnection = false
watch(
  () => chat.data,
  (data) => {
    if (!data || checkedFirstConnection) return
    checkedFirstConnection = true
    if (
      data.demo ||
      data.networks.length ||
      !connectionGuideEnabled ||
      new URL(location.href).searchParams.has('irc') ||
      overlay.value ||
      readLocal(setupKey, (value) => value === true)
    )
      return
    networkDialog.value = { id: '', firstConnection: true }
    overlay.value = 'network'
  },
  { immediate: true },
)
// ---- Layout, drawers and gestures -------------------------------------------------------------

// Match workspace-layout.css: members dock at ≥1100px, the channel list at ≥900px.
const MEMBERS_DOCKED = 1100
const SIDEBAR_DOCKED = 900
const mobileNav = ref(false)
const mobileMembers = ref(false)
const wideSidebar = useMediaQuery(`(min-width: ${SIDEBAR_DOCKED}px)`)
const wideMembers = useMediaQuery(`(min-width: ${MEMBERS_DOCKED}px)`)
const horizontalSegments = useMediaQuery('(horizontal-viewport-segments: 2)')
const verticalSegments = useMediaQuery('(vertical-viewport-segments: 2)')
const sidebarDocked = computed(() => horizontalSegments.value || wideSidebar.value)
const membersDocked = computed(
  () => wideMembers.value && !horizontalSegments.value && !verticalSegments.value,
)
const membersVisible = computed(
  () => !chat.serverView && chat.active?.kind === 'channel' && layout.state.showMembers,
)
/** The conversation's search panel, which takes the member list's place while open. */
const searchOpen = ref(false)
const searchAvailable = computed(() => !chat.serverView && !!chat.active)
const searching = computed(() => searchOpen.value && searchAvailable.value)
const membersOpen = computed(
  () => !searching.value && (membersDocked.value ? membersVisible.value : mobileMembers.value),
)
// Saved widths are at 100% interface scale, so a larger scale widens the panels with their text.
const appearance = useAppearanceStore()
const panelScale = computed(() => appearance.settings.interfaceScale / 100)
const gridStyle = computed(() => ({
  '--sidebar-w': `calc(${layout.state.sidebarWidth}px * var(--interface-scale, 1))`,
  '--members-w': `calc(${layout.state.membersWidth}px * var(--interface-scale, 1))`,
}))
function toggleMembers() {
  // From search, the members button switches the panel back to members.
  if (searching.value) {
    searchOpen.value = false
    if (!membersDocked.value) mobileMembers.value = true
    else layout.set('showMembers', true)
    return
  }
  if (!membersDocked.value) mobileMembers.value = !mobileMembers.value
  else layout.set('showMembers', !layout.state.showMembers)
}
/** Narrow screens show the panel in the members drawer. */
function toggleSearch() {
  // Dismissing a drawer keeps its query. The next tap reopens those results.
  if (searching.value && !membersDocked.value && !mobileMembers.value) {
    mobileMembers.value = true
    return
  }
  searchOpen.value = !searching.value
  if (!membersDocked.value) mobileMembers.value = searchOpen.value
}
const closeDrawers = () => (mobileNav.value = mobileMembers.value = false)
function closeNavigation(event: KeyboardEvent) {
  if (!mobileNav.value || event.defaultPrevented) return
  event.preventDefault()
  event.stopPropagation()
  mobileNav.value = false
}
// Safari can hand Tab focus to browser chrome. Escape must still dismiss the drawer when
// focus returns to the document, without dismissing a menu or dialog layered over it.
useEventListener(window, 'keydown', (event) => {
  if (
    event.key !== 'Escape' ||
    event.defaultPrevented ||
    (!mobileNav.value && !mobileMembers.value) ||
    dialogs.count ||
    document.querySelector('[role="menu"], [role="listbox"], .expression-picker')
  )
    return
  event.preventDefault()
  closeDrawers()
})
let drawerOpener: HTMLElement | null = null
watch([mobileNav, mobileMembers], async ([nav, members], [wasNav, wasMembers]) => {
  if ((nav || members) && !wasNav && !wasMembers)
    drawerOpener = document.activeElement instanceof HTMLElement ? document.activeElement : null
  await nextTick()
  if (nav || members) {
    const panel = document.querySelector<HTMLElement>(
      nav ? '.conversation-sidebar' : '.member-sidebar',
    )
    if (panel && !panel.contains(document.activeElement))
      panel
        .querySelector<HTMLElement>('input, button:not(:disabled)')
        ?.focus({ preventScroll: true })
  } else if (
    drawerOpener?.isConnected &&
    (document.activeElement === document.body ||
      document.activeElement?.closest('.conversation-sidebar, .member-sidebar'))
  ) {
    drawerOpener.focus({ preventScroll: true })
    drawerOpener = null
  }
})
useTouchGestures({
  openNav: () => {
    if (!sidebarDocked.value) mobileNav.value = true
  },
  openMembers: () => {
    if (!membersDocked.value && chat.active?.kind === 'channel' && !chat.serverView)
      mobileMembers.value = true
  },
  closeDrawers,
  drawerOpen: () => mobileNav.value || mobileMembers.value,
})
useWorkspaceShortcuts({
  escape: () => {
    closeDrawers()
    replyTo.value = undefined
  },
  overlay: () => overlay.value,
  // Closing through the dialog's own close path restores focus.
  close: () => dialogs.closeTop() || (overlay.value = null),
  toggleMembers,
})
watch([sidebarDocked, membersDocked], ([sidebar, members]) => {
  if (sidebar) mobileNav.value = false
  if (members) mobileMembers.value = false
})
const drop = useFileDrop()

// ---- Navigation, history, notifications, links ------------------------------------------------

const { attention } = useNotifications()
// Highlights read aloud with your speech service, when this device has it on (#201).
useHighlightReader()
// Activity reports let the server route alerts to the device in use (#84).
useActivity()
// Round trips to IRCord for the server page, sampled in the background so its chart has history.
useClientLinkSampler()
const { visit } = useRecentConversations()
// Address bar ↔ open conversation, so back/forward switch conversations.
useUrlSync()
const ircLinks = useIrcLinks()
// irc:// links handed over by the browser arrive as ?irc=<url>.
const stopIrc = watch(
  () => chat.data,
  (data) => {
    if (!data) return
    queueMicrotask(() => stopIrc())
    const link = new URL(location.href).searchParams.get('irc')
    if (!link) return
    window.history.replaceState(null, '', location.pathname + location.hash)
    void ircLinks.open(link)
  },
  { immediate: true },
)
watch(
  () => chat.activeId,
  (id) => {
    replyTo.value = undefined
    closeDrawers()
    visit(id)
  },
  { immediate: true },
)

const title = computed(() =>
  chat.serverView
    ? chat.network?.name
    : chat.active?.kind === 'direct'
      ? (chat.directPartner(chat.active)?.displayName ?? chat.active.name)
      : chat.active?.name,
)
provideWorkspacePane({
  title,
  replyTo,
  highlightId,
  composer,
  mobileNav,
  mobileMembers,
  membersVisible,
  membersOpen,
  toggleMembers,
  searching,
  toggleSearch,
  requestReply,
  drop,
})
watch(() => route.path, closeDrawers)
const pageTitle = computed(() => workspacePages[route.path as keyof typeof workspacePages]?.title)
useHead({
  title: () =>
    `${attention.value ? `(${attention.value}) ` : ''}${
      pageTitle.value
        ? `${pageTitle.value} · IRCord`
        : title.value
          ? `${!chat.serverView && chat.active?.kind === 'channel' ? '#' : ''}${title.value} · IRCord`
          : 'IRCord'
    }`,
})
onBeforeUnmount(() => {
  clearTimeout(highlightTimer)
})
</script>
<template>
  <div
    class="workspace grid overflow-hidden h-[var(--app-height,100dvh)] pt-[env(safe-area-inset-top)] pr-[env(safe-area-inset-right)] pb-0 pl-[env(safe-area-inset-left)] [touch-action:pan-y_pinch-zoom]"
    :class="{
      'mobile-navigation-open': mobileNav,
      'mobile-members-open': mobileMembers,
      'members-hidden': !membersVisible && !searching,
      'sidebar-hidden': !layout.state.showSidebar,
    }"
    :style="gridStyle"
  >
    <a
      class="skip-link fixed z-100 top-[-100px] left-4 bg-primary text-primary-foreground py-2.5 px-3.5 rounded-md focus:top-3"
      href="#message-input"
      >Skip to message input</a
    >
    <BootScreen v-if="chat.bootstrap.isLoading && !chat.data" />
    <BootScreen
      v-else-if="chat.bootstrap.error && !chat.data"
      title="Couldn’t load your workspace"
      :detail="errorText(chat.bootstrap.error)"
      @retry="chat.bootstrap.refetch()"
    />
    <template v-else-if="chat.data">
      <button
        v-if="mobileNav || mobileMembers"
        class="mobile-scrim hidden"
        :aria-label="
          mobileNav ? 'Close conversations' : searching ? 'Close search' : 'Close members'
        "
        @click="closeDrawers"
      />
      <WorkspaceBar :inert="mobileNav || mobileMembers" />
      <FocusScope
        as-child
        :present="mobileNav"
        :trapped="mobileNav"
        :loop="mobileNav"
        @mount-auto-focus.prevent
        @unmount-auto-focus.prevent
        ><ConversationSidebar
          :inert="mobileMembers"
          :role="mobileNav ? 'dialog' : undefined"
          :aria-modal="mobileNav || undefined"
          @search="actions.search()"
          @drafts="actions.drafts()"
          @join="actions.join()"
          @close="mobileNav = false"
          @keydown.esc="closeNavigation"
        >
          <ResizeHandle
            label="Resize conversation list"
            direction="right"
            :value="layout.state.sidebarWidth"
            :min="layoutLimits.sidebar.min"
            :max="layoutLimits.sidebar.max"
            :default="layoutLimits.sidebar.default"
            :scale="panelScale"
            @resize="layout.set('sidebarWidth', $event)"
          /> </ConversationSidebar
      ></FocusScope>
      <main
        class="chat-main flex flex-col min-w-0 min-h-0 bg-background relative"
        :inert="mobileNav"
      >
        <NotificationSetupPrompt
          :busy="
            !!overlay ||
            !isConversationPath(route.path) ||
            mobileNav ||
            mobileMembers ||
            !!prompt.current.value
          "
        />
        <slot />
      </main>
      <div
        v-if="chat.notice"
        class="toast fixed bottom-22 left-1/2 [transform:translateX(-50%)] z-50 flex items-center gap-2 pt-1.5 pr-1.5 pb-1.5 pl-3.5 bg-secondary shadow-popover text-foreground rounded-md text-[13px] max-w-[calc(100%_-_32px)] max-[700px]:bottom-[calc(76px_+_env(safe-area-inset-bottom))]"
        @pointerenter="chat.holdNotice(true)"
        @pointerleave="chat.holdNotice(false)"
        @focusin="chat.holdNotice(true)"
        @focusout="chat.holdNotice(false)"
        @keydown.esc="dismissNotice"
      >
        <!-- Announced through useStatusAnnouncer, so this is not a live region of its own. -->
        <span>{{ chat.notice }}</span>
        <UiButton
          variant="ghost"
          size="icon-sm"
          class="size-6"
          aria-label="Dismiss notification"
          @click="dismissNotice"
        >
          <AppIcon
            name="close"
            :size="14"
          />
        </UiButton>
      </div>
      <SettingsDialog
        v-if="overlay === 'settings'"
        :initial-tab="settingsTab"
        @close="overlay = null"
        @add-network="actions.addNetwork()"
        @edit-network="actions.editNetwork($event)"
      />
      <AdministrationDialog
        v-else-if="overlay === 'administration'"
        @close="overlay = null"
      />
      <ChannelSettings
        v-else-if="overlay === 'channel' && channelPanel.id"
        :conversation-id="channelPanel.id"
        :initial-tab="channelPanel.tab"
        @close="overlay = null"
      />
      <NetworkDialog
        v-else-if="overlay === 'network'"
        :network-id="networkDialog.id"
        :prefill="networkDialog.prefill"
        :first-connection="networkDialog.firstConnection"
        @close="closeNetworkDialog"
      />
      <JoinDialog
        v-else-if="overlay === 'join'"
        @close="overlay = null"
      />
      <ProfileDialog
        v-else-if="overlay === 'profile'"
        :member-id="profileId"
        @close="overlay = null"
        @edit="actions.settings('profile')"
      />
      <AppDialog
        v-else-if="overlay === 'shortcuts'"
        title="Keyboard shortcuts"
        wide
        @close="overlay = null"
        ><div class="dialog-form pt-0 pr-5 pb-5 pl-5">
          <ShortcutList @customize="actions.settings('shortcuts')" /></div
      ></AppDialog>
      <ContextMenu />
      <PromptDialog />
    </template>
  </div>
</template>
