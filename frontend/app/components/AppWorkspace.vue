<script setup lang="ts">
import type { Message, NetworkInput } from '../../shared/types/chat'
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
  'search' | 'settings' | 'network' | 'join' | 'profile' | 'image' | 'channel' | 'shortcuts'
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
const image = ref({ url: '', alt: '' })
const replyTo = ref<Message>()
const highlightId = ref('')
let highlightTimer: ReturnType<typeof setTimeout> | undefined

function highlight(messageId: string) {
  overlay.value = null
  highlightId.value = messageId
  clearTimeout(highlightTimer)
  highlightTimer = setTimeout(() => (highlightId.value = ''), 3500)
}
function jump(conversationId: string, messageId: string) {
  chat.activate(conversationId)
  highlight(messageId)
}

const historyTools = useHistoryTools()
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
actions.shortcuts = () => (overlay.value = 'shortcuts')
actions.reply = (message) => (replyTo.value = message)
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
function viewImage(url: string, alt: string) {
  image.value = { url, alt }
  overlay.value = 'image'
}

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
})
function resized() {
  if (window.innerWidth >= MEMBERS_DOCKED) mobileMembers.value = false
  if (window.innerWidth >= SIDEBAR_DOCKED) mobileNav.value = false
}
const drop = useFileDrop()

// ---- Navigation, history, notifications, links ------------------------------------------------

const { attention } = useNotifications()
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
      <span class="brand-mark"
        ><AppIcon
          name="brand"
          :size="26"
      /></span>
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
          @reply="replyTo = $event"
          @profile="actions.profile"
          @image="viewImage"
        />
        <div id="message-input">
          <ChatComposer
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
      <LazySettingsDialog
        v-else-if="overlay === 'settings'"
        :initial-tab="settingsTab"
        @close="overlay = null"
        @add-network="actions.addNetwork()"
        @edit-network="actions.editNetwork($event)"
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
      <AppDialog
        v-else-if="overlay === 'image'"
        :title="image.alt || 'Image'"
        image
        @close="overlay = null"
        ><img
          class="lightbox-image"
          :src="image.url"
          :alt="image.alt"
      /></AppDialog>
      <ContextMenu />
      <PromptDialog />
    </template>
  </div>
</template>
