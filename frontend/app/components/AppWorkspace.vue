<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type {
  InboxItem,
  LinkSource,
  Message,
  NetworkInput,
  SavedItem,
} from '../../shared/types/chat'
import { messagesQuery } from '../queries/chat'
import { layoutLimits } from '../stores/layout'

/**
 * The signed-in app: layout (rail, sidebars, conversation), drawers on small screens, and the
 * overlays everything else opens through useWorkspaceActions().
 */
const chat = useChatStore()
const layout = useLayoutStore()
const auth = useAuthStore()
const connectionGuideEnabled = useRuntimeConfig().public.firstConnectionGuide
usePreferencesStore()
// Which messages are saved, for every message's toolbar and menu.
useSavedStore()
// The inbox's unread indicator in the header.
useInboxStore()
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
  | 'search'
  | 'drafts'
  | 'saved'
  | 'links'
  | 'inbox'
  | 'settings'
  | 'network'
  | 'join'
  | 'profile'
  | 'channel'
  | 'shortcuts'
  | 'administration'
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
function resumeDraft(conversationId: string) {
  chat.activate(conversationId)
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
function jump(conversationId: string, messageId: string) {
  chat.activate(conversationId)
  highlight(messageId)
}

const historyTools = useHistoryTools()
/** Opens a saved message where it was sent; the conversation you leave keeps its draft. */
async function openSaved(item: SavedItem) {
  overlay.value = null
  if (await historyTools.reveal(item.conversationId, item.messageId))
    highlight(item.messageId)
}
/** Opens a link's post where it was sent; the conversation you leave keeps its draft. */
async function openLinkSource(source: LinkSource) {
  overlay.value = null
  if (await historyTools.reveal(source.conversationId, source.messageId))
    highlight(source.messageId)
}
/**
 * Opens an inbox item in its conversation, loading history around it when needed. This is
 * the ordinary conversation view, so the usual read rules apply; the inbox itself marks nothing.
 */
async function openInbox(item: InboxItem) {
  overlay.value = null
  if (await historyTools.reveal(item.conversationId, item.messageId))
    highlight(item.messageId)
}
const actions = useWorkspaceActions()
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
actions.search = () => (overlay.value = overlay.value === 'search' ? null : 'search')
actions.drafts = () => (overlay.value = 'drafts')
actions.saved = () => (overlay.value = overlay.value === 'saved' ? null : 'saved')
/** The channel the links view opened from; empty opens every channel's links. */
const linksFrom = ref('')
actions.links = (conversationId = '') => {
  linksFrom.value = conversationId
  overlay.value = overlay.value === 'links' ? null : 'links'
}
actions.inbox = () => (overlay.value = overlay.value === 'inbox' ? null : 'inbox')
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

// Breakpoints (match workspace.css): members dock at ≥1100px, the channel list at ≥900px.
const MEMBERS_DOCKED = 1100
const SIDEBAR_DOCKED = 900
const mobileNav = ref(false)
const mobileMembers = ref(false)
const membersVisible = computed(
  () => !chat.serverView && chat.active?.kind === 'channel' && layout.state.showMembers,
)
const gridStyle = computed(() => ({
  '--sidebar-w': `${layout.state.sidebarWidth}px`,
  '--members-w': `${layout.state.membersWidth}px`,
}))
function toggleMembers() {
  if (window.innerWidth < MEMBERS_DOCKED) mobileMembers.value = !mobileMembers.value
  else layout.set('showMembers', !layout.state.showMembers)
}
const closeDrawers = () => (mobileNav.value = mobileMembers.value = false)
useTouchGestures({
  openNav: () => {
    if (window.innerWidth < SIDEBAR_DOCKED) mobileNav.value = true
  },
  openMembers: () => {
    if (window.innerWidth < MEMBERS_DOCKED && chat.active?.kind === 'channel' && !chat.serverView)
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
  // Closing through the dialog element runs its close path, which restores focus.
  close: () => {
    const open = document.querySelector<HTMLDialogElement>('dialog[open]')
    if (open) open.close()
    else overlay.value = null
  },
})
function resized() {
  if (window.innerWidth >= MEMBERS_DOCKED) mobileMembers.value = false
  if (window.innerWidth >= SIDEBAR_DOCKED) mobileNav.value = false
}
const drop = useFileDrop()

// ---- Navigation, history, notifications, links ------------------------------------------------

const { attention } = useNotifications()
// Activity reports let the server route alerts to the device in use (#84).
useActivity()
const { visit } = useRecentConversations()
// Address bar ↔ open conversation, so back/forward switch conversations.
useUrlSync()
// The last workspace stays readable offline (installed app, flaky connections).
useOfflineSnapshot()
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
useHead({
  title: () =>
    `${attention.value ? `(${attention.value}) ` : ''}${
      title.value
        ? `${!chat.serverView && chat.active?.kind === 'channel' ? '#' : ''}${title.value} · IRCord`
        : 'IRCord'
    }`,
})
onMounted(() => window.addEventListener('resize', resized))
onBeforeUnmount(() => {
  window.removeEventListener('resize', resized)
  clearTimeout(highlightTimer)
})
</script>
<template>
  <div
    class="workspace"
    :class="{
      'mobile-navigation-open': mobileNav,
      'mobile-members-open': mobileMembers,
      'members-hidden': !membersVisible,
    }"
    :style="gridStyle"
  >
    <a
      class="skip-link"
      href="#message-input"
      >Skip to message input</a
    >
    <div
      v-if="chat.bootstrap.isLoading && !chat.data"
      class="boot-screen"
    >
      <BrandMark class="brand-mark" />
      <div class="loading-track"><span /></div>
    </div>
    <div
      v-else-if="chat.bootstrap.error && !chat.data"
      class="boot-screen"
    >
      <AppIcon
        name="offline"
        :size="30"
      />
      <h1>Couldn’t load your workspace</h1>
      <p>{{ errorText(chat.bootstrap.error) }}</p>
      <button
        class="button primary"
        @click="chat.bootstrap.refetch()"
      >
        <AppIcon
          name="refresh"
          :size="16"
        />Try again
      </button>
    </div>
    <template v-else-if="chat.data">
      <button
        v-if="mobileNav || mobileMembers"
        class="mobile-scrim"
        aria-label="Close navigation"
        @click="closeDrawers"
      />
      <NetworkRail @add="actions.addNetwork()" />
      <ConversationSidebar
        @search="actions.search()"
        @drafts="actions.drafts()"
        @join="actions.join()"
        @settings="actions.settings()"
        @profile="actions.profile"
      >
        <ResizeHandle
          label="Resize conversation list"
          direction="right"
          :value="layout.state.sidebarWidth"
          :min="layoutLimits.sidebar.min"
          :max="layoutLimits.sidebar.max"
          :default="layoutLimits.sidebar.default"
          @resize="layout.set('sidebarWidth', $event)"
        />
      </ConversationSidebar>
      <main
        class="chat-main"
        :inert="mobileNav || mobileMembers"
        @dragenter="drop.enter"
        @dragleave="drop.leave"
        @dragover.prevent
        @drop.prevent="drop.drop"
      >
        <div
          v-if="drop.dragging.value"
          class="drop-overlay"
        >
          <span
            ><AppIcon
              name="upload"
              :size="32"
            />Drop to attach to
            {{ chat.active?.kind === 'channel' ? `#${chat.active.name}` : title }}</span
          >
        </div>
        <ChatHeader
          :title="title"
          :members-visible="membersVisible"
          @open-nav="mobileNav = true"
          @toggle-members="toggleMembers"
        />
        <ConnectionBanners />
        <InvitationBanner />
        <ServerWindow v-if="chat.serverView" />
        <ChatTimeline
          v-else
          :highlight-id="highlightId"
          @reply="requestReply"
          @profile="actions.profile"
        />
        <div id="message-input">
          <ChatComposer
            ref="composer"
            :reply-to="replyTo"
            @cancel-reply="replyTo = undefined"
          />
        </div>
      </main>
      <MemberSidebar
        v-if="membersVisible || mobileMembers"
        @profile="actions.profile"
        @close="mobileMembers = false"
      >
        <ResizeHandle
          label="Resize member list"
          direction="left"
          :value="layout.state.membersWidth"
          :min="layoutLimits.members.min"
          :max="layoutLimits.members.max"
          :default="layoutLimits.members.default"
          @resize="layout.set('membersWidth', $event)"
        />
      </MemberSidebar>
      <div
        v-if="chat.notice"
        class="toast"
        role="status"
      >
        <span>{{ chat.notice }}</span>
        <button
          class="icon-button"
          aria-label="Dismiss notification"
          @click="chat.notice = ''"
        >
          <AppIcon
            name="close"
            :size="14"
          />
        </button>
      </div>
      <LazySearchDialog
        v-if="overlay === 'search'"
        @close="overlay = null"
        @jump="jump"
      />
      <LazyDraftsDialog
        v-else-if="overlay === 'drafts'"
        @close="overlay = null"
        @open="resumeDraft"
      />
      <LazySavedDialog
        v-else-if="overlay === 'saved'"
        @close="overlay = null"
        @open="openSaved"
      />
      <LazyLinksDialog
        v-else-if="overlay === 'links'"
        :conversation-id="linksFrom"
        @close="overlay = null"
        @open="openLinkSource"
      />
      <LazyInboxDialog
        v-else-if="overlay === 'inbox'"
        @close="overlay = null"
        @open="openInbox"
      />
      <LazySettingsDialog
        v-else-if="overlay === 'settings'"
        :initial-tab="settingsTab"
        @close="overlay = null"
        @add-network="actions.addNetwork()"
        @edit-network="actions.editNetwork($event)"
      />
      <LazyAdministrationDialog
        v-else-if="overlay === 'administration'"
        @close="overlay = null"
      />
      <LazyChannelSettings
        v-else-if="overlay === 'channel' && channelPanel.id"
        :conversation-id="channelPanel.id"
        :initial-tab="channelPanel.tab"
        @close="overlay = null"
      />
      <LazyNetworkDialog
        v-else-if="overlay === 'network'"
        :network-id="networkDialog.id"
        :prefill="networkDialog.prefill"
        :first-connection="networkDialog.firstConnection"
        @close="closeNetworkDialog"
      />
      <LazyJoinDialog
        v-else-if="overlay === 'join'"
        @close="overlay = null"
      />
      <LazyProfileDialog
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
        ><div class="dialog-form"><ShortcutList /></div
      ></AppDialog>
      <ContextMenu />
      <PromptDialog />
    </template>
  </div>
</template>
