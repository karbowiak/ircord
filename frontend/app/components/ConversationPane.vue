<script setup lang="ts">
import { layoutLimits } from '../stores/layout'
import type { ComponentPublicInstance } from 'vue'
import { FocusScope } from 'reka-ui'
const props = defineProps<{ kind?: 'channel' | 'direct' | 'server' }>()
const chat = useChatStore()
const layout = useLayoutStore()
const appearance = useAppearanceStore()
const actions = useWorkspaceActions()
const {
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
} = useWorkspacePane()
const available = computed(() =>
  !props.kind
    ? !!chat.active || (!!chat.network && chat.serverView)
    : props.kind === 'server'
      ? !!chat.network && chat.serverView
      : !!chat.active && !chat.serverView && (!props.kind || chat.active.kind === props.kind),
)
// During a route transition, the previous composer can unmount after its replacement mounts.
let ownComposer: { resume: () => boolean } | null = null
function setComposer(instance: Element | ComponentPublicInstance | null) {
  if (instance) {
    ownComposer = instance as ComponentPublicInstance & { resume: () => boolean }
    composer.value = ownComposer
  } else if (composer.value === ownComposer) composer.value = null
}
const emptyTitle = computed(() =>
  props.kind === 'server' ? 'Server' : props.kind === 'direct' ? 'Direct messages' : 'Channels',
)
function closeMembers(event: KeyboardEvent) {
  if (!mobileMembers.value || event.defaultPrevented) return
  event.preventDefault()
  event.stopPropagation()
  mobileMembers.value = false
}
</script>
<template>
  <section
    class="conversation-pane flex min-h-0 min-w-0 flex-1 flex-col"
    @dragenter="available && drop.enter($event)"
    @dragleave="drop.leave"
    @dragover.prevent
    @drop.prevent="available && drop.drop($event)"
  >
    <template v-if="available">
      <div
        v-if="drop.dragging.value"
        class="drop-overlay absolute inset-2 z-30 grid place-items-center rounded-lg border-2 border-dashed border-primary bg-background/90 text-[16px] font-semibold pointer-events-none"
      >
        <span class="grid justify-items-center gap-2"
          ><AppIcon
            name="upload"
            :size="32"
          />Drop to attach to
          {{ chat.active?.kind === 'channel' ? `#${chat.active.name}` : title }}</span
        >
      </div>
      <ChatHeader
        :title="title"
        :members-visible="membersOpen"
        :searching="searching"
        :inert="mobileMembers"
        @open-nav="mobileNav = true"
        @toggle-members="toggleMembers"
        @toggle-search="toggleSearch"
      />
      <!-- Search results need more room than the member list it replaces. -->
      <div
        class="conversation-body grid min-h-0 flex-1"
        :style="
          searching ? { '--members-w': 'calc(340px * var(--interface-scale, 1))' } : undefined
        "
      >
        <section
          class="chat-content flex min-h-0 min-w-0 flex-col"
          :inert="mobileMembers"
          aria-label="Conversation"
        >
          <div class="conversation-history flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
            <ConnectionBanners /><InvitationBanner />
            <ServerWindow v-if="chat.serverView" />
            <ChatTimeline
              v-else
              :highlight-id="highlightId"
              @reply="requestReply"
              @profile="actions.profile"
            />
          </div>
          <div id="message-input">
            <ChatComposer
              :ref="setComposer"
              :reply-to="replyTo"
              @cancel-reply="replyTo = undefined"
            />
          </div>
        </section>
        <FocusScope
          v-if="searching || membersVisible || mobileMembers"
          :key="`${searching ? 'search' : 'members'}:${chat.activeId}`"
          as-child
          :present="mobileMembers"
          :trapped="mobileMembers"
          :loop="mobileMembers"
          @mount-auto-focus.prevent
          @unmount-auto-focus.prevent
          ><ConversationSearch
            v-if="searching && chat.active"
            :key="chat.active.id"
            :role="mobileMembers ? 'dialog' : undefined"
            :aria-modal="mobileMembers || undefined"
            :conversation="chat.active"
            @close="toggleSearch" />
          <MemberSidebar
            v-else-if="membersVisible || mobileMembers"
            :role="mobileMembers ? 'dialog' : undefined"
            :aria-modal="mobileMembers || undefined"
            @profile="actions.profile"
            @close="mobileMembers = false"
            @keydown.esc="closeMembers"
          >
            <ResizeHandle
              label="Resize member list"
              direction="left"
              :value="layout.state.membersWidth"
              :min="layoutLimits.members.min"
              :max="layoutLimits.members.max"
              :default="layoutLimits.members.default"
              :scale="appearance.settings.interfaceScale / 100"
              @resize="layout.set('membersWidth', $event)"
            /> </MemberSidebar
        ></FocusScope>
      </div>
    </template>
    <WorkspacePage
      v-else
      :title="emptyTitle"
      :icon="kind === 'server' ? 'terminal' : kind === 'direct' ? 'message' : 'hash'"
      @close="actions.search()"
    >
      <EmptyState class="flex-1"
        ><p>
          {{
            kind === 'server'
              ? 'Add a network to open its server window.'
              : kind === 'direct'
                ? 'Choose a direct message or find someone to talk to.'
                : 'Choose a channel from the sidebar or join one.'
          }}
        </p>
        <UiButton
          variant="secondary"
          @click="
            kind === 'server'
              ? actions.addNetwork()
              : kind === 'direct'
                ? actions.search()
                : actions.join()
          "
          >{{
            kind === 'server'
              ? 'Add a network'
              : kind === 'direct'
                ? 'Find someone'
                : 'Join a channel'
          }}</UiButton
        ></EmptyState
      >
    </WorkspacePage>
  </section>
</template>
