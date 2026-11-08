<script setup lang="ts">
import type { ViewerItem, Message } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import { stripFormatting } from '../../shared/utils/irc'
import { attractClock } from '../../shared/utils/attract'
const props = defineProps<{
  message: Message
  grouped?: boolean
  highlighted?: boolean
  /** /attract: shine across this row, sparkle on its avatar, or both. */
  attract?: 'row' | 'avatar' | 'both'
  /** When the /attract effect started, so a row mounted mid-effect joins it in step. */
  attractStartedAt?: number
}>()
const emit = defineEmits<{
  reply: [message: Message]
  profile: [id: string]
  image: [url: string, alt: string, set?: ViewerItem[]]
}>()
const chat = useChatStore()
const appearance = useAppearanceStore()
const library = useLibraryStore()
const nameStyle = useNameStyle()
const { time, full } = useTimeFormat()
const reacting = ref(false)
// Computed once per effect: re-renders must not move a running animation's delay.
const attractStyle = computed(() =>
  props.attract && props.attractStartedAt ? attractClock(props.attractStartedAt) : undefined,
)
const reactButton = ref<HTMLButtonElement | null>(null)
const author = computed(() => chat.member(props.message.senderId))
const notifications = useNotificationStore()
/** Your nick on this network, or one of your highlight words. */
const mentionsMe = computed(() => {
  if (props.message.senderId === 'you') return false
  const c = chat.data?.conversations.find((x) => x.id === props.message.conversationId)
  return notifications.highlights(
    props.message.text,
    (c && chat.networkOf(c)?.nick) ?? chat.data?.user.nick,
  )
})
const irc = computed(() => appearance.settings.messageLayout === 'irc')
const action = computed(() => props.message.kind === 'action')
/** An upload's message text is just its URL; the attachment card already shows it. */
const showText = computed(
  () =>
    !props.message.gif &&
    !(
      props.message.attachment &&
      /^https?:\/\/\S+$/.test(props.message.text) &&
      props.message.text.endsWith(props.message.attachment.url.replace(/^https?:\/\/[^/]+/, ''))
    ),
)
/** The three most recently used emoji (defaults until there is history). */
const quickReactions = computed(() => library.quickReactions.slice(0, 3))
const menus = useMenus()
const saved = useSavedStore()
const isSaved = computed(() => saved.isSaved(props.message.id))
const saveChange = computed(() => saved.pendingChange(props.message.id))
const saveLabel = computed(() =>
  saveChange.value === 'saving'
    ? 'Saving…'
    : saveChange.value === 'removing'
      ? 'Removing from saved…'
      : isSaved.value
        ? 'Remove from saved'
        : 'Save for later',
)
const reactionSupported = computed(() => chat.messageActionSupported(props.message, 'reactions'))
const replySupported = computed(() => chat.messageActionSupported(props.message, 'replies'))
const reactionReason = computed(() => chat.messageActionReason(props.message, 'reactions'))
const replyReason = computed(() => chat.messageActionReason(props.message, 'replies'))
watch(reactionReason, (reason) => {
  if (reason) reacting.value = false
})
const authorMenu = (event: MouseEvent) => author.value && menus.member(event, author.value)
const row = ref<HTMLElement | null>(null)
/** The reaction picker opens from the toolbar button, or from the row (long-press menu). */
const reactAnchor = ref<HTMLElement | null>(null)
function openReactions(anchor: HTMLElement | null = row.value) {
  if (reactionReason.value) return
  reactAnchor.value = anchor
  reacting.value = !!anchor
}
/** Present for your own messages when this network negotiated redaction. */
const removal = computed(() => menus.deletion(props.message))
/** Present for your own text messages; disabled with the reason where editing is unavailable. */
const editable = computed(() => menus.editing(props.message))
const messageEditing = useMessageEditing()
const editing = computed(() => messageEditing.editingId.value === props.message.id)
const showEdits = ref(false)
/** Every version before the current text, oldest first. */
const previousVersions = computed(() => {
  const edits = props.message.edits ?? []
  if (!edits.length) return []
  return [
    { text: props.message.originalText ?? '', at: props.message.createdAt },
    ...edits.slice(0, -1).map((e) => ({ text: e.text, at: e.at })),
  ]
})
const messageMenu = (event: MouseEvent | HTMLElement) =>
  menus.message(event, props.message, () => openReactions())
const moreButton = ref<HTMLButtonElement | null>(null)
const unreadable = computed(() => menus.unreadable(props.message))
const contextMenu = useContextMenu()
/** Names the Shift actions this message actually offers. */
const moreTitle = computed(() => {
  const hidden = [unreadable.value && 'Mark as unread', removal.value && 'Delete'].filter(Boolean)
  return alternate.value || !hidden.length
    ? 'More actions'
    : `More actions (hold Shift for ${hidden.join(' and ')})`
})

// Holding Shift swaps in the alternate actions while the pointer is over the message, or
// while focus is in it, except keyboard focus on the toolbar itself (so Shift+Tab through
// its buttons never yanks the focused one away).
const shift = useShiftHeld()
const hovered = ref(false)
const focus = ref<'none' | 'row' | 'toolbar'>('none')
function trackFocus(event: FocusEvent) {
  // Focus moving within the message keeps the current mode until its focusin arrives;
  // a render in between must not see “no focus” and swap the toolbar under the move.
  const next = event.relatedTarget
  if (event.type === 'focusout' && next instanceof Node && row.value?.contains(next)) return
  const target = event.type === 'focusin' ? (event.target as Element) : null
  // A clicked button keeps focus too; only keyboard focus (focus-visible) pins the toolbar.
  focus.value = !target
    ? 'none'
    : target.closest('.message-actions') && target.matches(':focus-visible')
      ? 'toolbar'
      : 'row'
}
const alternate = computed(
  () => shift.value && focus.value !== 'toolbar' && (hovered.value || focus.value === 'row'),
)
watch(
  () => hovered.value || focus.value !== 'none',
  (active) => active && library.loadQuickReactions(),
)

const emojiLabel = (item: EmojiItem) =>
  item.unicode ? emojiText(item, library.library.skinTone) : `:${item.shortcode}:`
function react(item: EmojiItem) {
  reacting.value = false
  void menus.reactWith(props.message, item)
}
</script>
<template>
  <article
    class="message-row"
    :style="attractStyle"
    :class="{
      'is-grouped': grouped,
      'is-highlighted': highlighted,
      'is-attract': attract === 'row' || attract === 'both',
      'is-mention': mentionsMe,
      'is-pending': message.delivery === 'pending',
      'is-failed': message.delivery === 'failed',
      'is-mine': message.senderId === 'you',
      'is-action': action,
      'is-notice': message.kind === 'notice',
      'is-deleted': message.deleted,
      'is-editing': editing,
      'show-actions': reacting,
    }"
    ref="row"
    :aria-label="`${author?.displayName}, ${time(message.createdAt)}`"
    @contextmenu="messageMenu($event)"
    @pointerover="hovered = $event.pointerType !== 'touch'"
    @pointerleave="hovered = false"
    @focusin="trackFocus"
    @focusout="trackFocus"
    @pointerdown="focus !== 'none' && (focus = 'row')"
  >
    <div
      v-if="message.replyTo"
      class="message-reply"
    >
      <AppIcon
        name="reply"
        :size="12"
      /><span class="reply-nick">{{ message.replyTo.nick }}</span
      ><span class="reply-excerpt">{{ stripFormatting(message.replyTo.text) }}</span>
    </div>

    <div
      v-if="irc"
      class="irc-line"
    >
      <time
        :datetime="message.createdAt"
        :title="full(message.createdAt)"
        >{{ time(message.createdAt) }}</time
      ><button
        class="author-name"
        :class="{ 'is-action': action, 'is-notice': message.kind === 'notice' }"
        :style="nameStyle(author)"
        @click="emit('profile', message.senderId)"
        @contextmenu.stop="authorMenu"
      >
        {{ author?.nick ?? author?.displayName }}
      </button>
      <div class="message-body">
        <em
          v-if="message.deleted"
          class="message-text message-deleted"
          >Message deleted</em
        >
        <!-- A div, not a span: code blocks put a <pre> in here. -->
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText"
          class="message-text"
        >
          <MessageContent
            :text="message.text"
            code-blocks
          /><MessageEdited
            v-if="message.editedAt"
            v-model:open="showEdits"
            :message="message"
          />
        </div>
        <GifMessage
          v-else-if="message.gif"
          :gif="message.gif"
          :media="message.media"
          @image="(url, alt) => emit('image', url, alt)"
        />
      </div>
    </div>

    <div
      v-else
      class="message-grid"
    >
      <button
        v-if="!grouped"
        class="avatar-button"
        :aria-label="`View ${author?.displayName}’s profile`"
        @click="emit('profile', message.senderId)"
        @contextmenu.stop="authorMenu"
      >
        <UserAvatar
          :member="author"
          :attract="attract === 'avatar' || attract === 'both'"
        />
      </button>
      <time
        v-else
        class="grouped-time"
        :datetime="message.createdAt"
        :title="full(message.createdAt)"
        >{{ time(message.createdAt) }}</time
      >
      <div class="message-body">
        <div
          v-if="!grouped"
          class="message-meta"
        >
          <button
            class="author-name"
            :style="nameStyle(author)"
            @click="emit('profile', message.senderId)"
            @contextmenu.stop="authorMenu"
          >
            {{ author?.displayName }}</button
          ><span
            v-if="author?.role === 'bot'"
            class="bot-badge"
            >BOT</span
          ><time
            :datetime="message.createdAt"
            :title="full(message.createdAt)"
            >{{ time(message.createdAt) }}</time
          >
        </div>
        <p
          v-if="message.deleted"
          class="message-text message-deleted"
        >
          <em>Message deleted</em>
        </p>
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText"
          class="message-text"
        >
          <span
            v-if="action"
            class="action-nick"
            :style="nameStyle(author)"
            >{{ author?.nick }}
          </span>
          <MessageContent
            :text="message.text"
            code-blocks
          /><MessageEdited
            v-if="message.editedAt"
            v-model:open="showEdits"
            :message="message"
          />
        </div>
        <GifMessage
          v-else-if="message.gif"
          :gif="message.gif"
          :media="message.media"
          @image="(url, alt) => emit('image', url, alt)"
        />
        <MessageExtras
          v-if="!message.deleted"
          :message="message"
          @image="(url, alt, set) => emit('image', url, alt, set)"
        />
      </div>
    </div>

    <MessageExtras
      v-if="irc && !message.deleted"
      :message="message"
      @image="(url, alt, set) => emit('image', url, alt, set)"
    />

    <ol
      v-if="showEdits && previousVersions.length && !message.deleted"
      class="message-edit-history"
      aria-label="Earlier versions"
    >
      <li
        v-for="(version, i) in previousVersions"
        :key="i"
      >
        <time
          :datetime="version.at"
          :title="full(version.at)"
          >{{ i === 0 ? 'Original' : 'Edited' }} {{ time(version.at) }}</time
        >
        <MessageContent :text="version.text" />
      </li>
    </ol>
    <button
      v-if="!message.delivery"
      class="mobile-message-menu icon-button"
      aria-label="Message actions"
      aria-haspopup="menu"
      @click="messageMenu($event.currentTarget as HTMLElement)"
    >
      <AppIcon
        name="more"
        :size="16"
      />
    </button>
    <div
      v-if="!message.delivery && !message.deleted"
      class="message-actions"
      role="toolbar"
      :aria-label="alternate ? 'More message actions' : 'Message actions'"
    >
      <template v-if="!alternate">
        <template v-if="reactionSupported">
          <button
            v-for="item in quickReactions"
            :key="item.id"
            class="icon-button quick-reaction"
            :aria-label="`React with ${emojiLabel(item)}`"
            :disabled="!!reactionReason"
            :title="reactionReason ?? emojiLabel(item)"
            @click="react(item)"
          >
            <CustomEmoji
              v-if="item.url"
              :item="item"
              icon
            /><template v-else>{{ emojiLabel(item) }}</template>
          </button>
          <span class="action-divider" />
        </template>
        <button
          v-if="reactionSupported"
          ref="reactButton"
          class="icon-button"
          aria-label="Add reaction"
          :disabled="!!reactionReason"
          :title="reactionReason ?? 'Add reaction'"
          :aria-expanded="reacting"
          @click="reacting ? (reacting = false) : openReactions(reactButton)"
        >
          <AppIcon
            name="smilePlus"
            :size="17"
          /></button
        ><button
          v-if="replySupported"
          class="icon-button"
          aria-label="Reply to message"
          :disabled="!!replyReason"
          :title="replyReason ?? 'Reply'"
          @click="emit('reply', message)"
        >
          <AppIcon
            name="reply"
            :size="17"
          /></button
        ><button
          v-if="editable"
          class="icon-button"
          aria-label="Edit message"
          :disabled="!editable.allowed || editing"
          :title="editable.why ?? 'Edit'"
          @click="messageEditing.start(message)"
        >
          <AppIcon
            name="pencil"
            :size="16"
          /></button
        ><button
          class="icon-button"
          aria-label="Copy message"
          title="Copy text"
          @click="menus.copyMessage(message)"
        >
          <AppIcon
            name="copy"
            :size="16"
          /></button
        ><button
          class="icon-button"
          :class="{ 'is-saved': isSaved, 'is-saving': !!saveChange }"
          :aria-label="saveLabel"
          :aria-pressed="isSaved"
          :aria-busy="!!saveChange || undefined"
          :title="saveLabel"
          @click="saved.toggle(message)"
        >
          <AppIcon
            :name="isSaved ? 'bookmarkSaved' : 'bookmark'"
            :size="16"
          />
        </button>
      </template>
      <template v-else>
        <button
          v-if="unreadable"
          class="icon-button"
          aria-label="Mark as unread"
          title="Mark as unread"
          @click="chat.markUnread(message)"
        >
          <AppIcon
            name="unread"
            :size="16"
          /></button
        ><button
          v-if="removal"
          class="icon-button danger"
          aria-label="Delete message"
          :title="removal.why ?? 'Delete for everyone'"
          :disabled="!removal.allowed"
          @click="menus.deleteMessage(message)"
        >
          <AppIcon
            name="trash"
            :size="16"
          />
        </button>
      </template>
      <button
        ref="moreButton"
        class="icon-button"
        aria-label="More actions"
        aria-haspopup="menu"
        :aria-expanded="contextMenu.state.open && contextMenu.state.returnFocus === moreButton"
        :title="moreTitle"
        @click="moreButton && messageMenu(moreButton)"
      >
        <AppIcon
          name="more"
          :size="16"
        />
      </button>
    </div>
    <LazyExpressionPicker
      v-if="reacting && reactAnchor"
      :anchor="reactAnchor"
      tab="emoji"
      emoji-only
      @close="reacting = false"
      @emoji="react"
    />
  </article>
</template>
