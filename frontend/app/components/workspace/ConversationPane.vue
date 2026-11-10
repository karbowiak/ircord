<script setup vapor lang="ts">
import type { ComponentPublicInstance } from 'vue'
import ConversationPanel from './ConversationPanel.vue'
const props = defineProps<{ kind?: 'channel' | 'direct' | 'server' }>()
const { t } = useI18n()
const $t = t
const chat = useChatStore()
const actions = useWorkspaceActions()
const {
  title,
  replyTo,
  highlightId,
  composer,
  mobileNav,
  mobileMembers,
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
      : !!chat.active &&
        !chat.serverView &&
        (!props.kind ||
          (props.kind === 'direct'
            ? chat.active.kind === 'direct' || !!chat.active.party
            : chat.active.kind === props.kind && !chat.active.party)),
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
  props.kind === 'server'
    ? t('conversation.server')
    : props.kind === 'direct'
      ? t('conversation.directMessages')
      : t('conversation.channels'),
)
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
          />{{
            $t('conversation.dropToAttach', {
              name:
                chat.active?.kind === 'channel' && !chat.active.party
                  ? `#${chat.active.name}`
                  : title,
            })
          }}</span
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
          :aria-label="$t('conversation.label')"
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
          <div
            id="message-input"
            class="relative"
          >
            <ConnectionBanners placement="composer" />
            <ChatComposer
              :ref="setComposer"
              :reply-to="replyTo"
              @cancel-reply="replyTo = undefined"
            />
          </div>
        </section>
        <ConversationPanel />
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
              ? $t('conversation.emptyServer')
              : kind === 'direct'
                ? $t('conversation.emptyDirect')
                : $t('conversation.emptyChannel')
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
              ? $t('networks.add')
              : kind === 'direct'
                ? $t('conversation.findSomeone')
                : $t('join.title')
          }}</UiButton
        ></EmptyState
      >
    </WorkspacePage>
  </section>
</template>
