<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
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
/** Your nick on this network, one of your highlight words, or a group mention including you. */
const mentionsMe = computed(() => {
  if (props.message.senderId === 'you') return false
  const c = chat.data?.conversations.find((x) => x.id === props.message.conversationId)
  return notifications.highlights(
    props.message.text,
    c ? chat.recipientFor(c, props.message) : chat.data?.user.nick,
  )
})
const irc = computed(() => appearance.settings.messageLayout === 'irc')
const action = computed(() => props.message.kind === 'action')
/** An upload's message text is just its URL; the attachment card already shows it. */
const showText = computed(
  () =>
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
    class="message-row group/message relative pt-0.5 pr-12 pb-0.5 pl-4 mt-[var(--group-gap)] text-[length:var(--message-size)] [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
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
      class="message-reply flex items-center gap-1.5 pt-0 pr-0 pb-0.5 pl-13.5 text-faint text-[12px] min-w-0 relative"
    >
      <AppIcon
        name="reply"
        :size="12"
      /><span class="reply-nick text-muted-foreground font-[600]">{{ message.replyTo.nick }}</span
      ><span class="reply-excerpt overflow-hidden whitespace-nowrap text-ellipsis min-w-0">{{
        stripFormatting(message.replyTo.text)
      }}</span>
    </div>

    <div
      v-if="irc"
      class="irc-line grid [grid-template-columns:auto_128px_minmax(0,_1fr)] gap-x-2.5 items-baseline font-chat text-[length:var(--message-size)] leading-[1.5]"
    >
      <time
        :datetime="message.createdAt"
        :title="full(message.createdAt)"
        >{{ time(message.createdAt) }}</time
      ><button
        class="author-name text-[15px] font-[600] leading-[1.375] overflow-hidden text-ellipsis whitespace-nowrap max-w-full hover:underline"
        :class="{ 'is-action': action, 'is-notice': message.kind === 'notice' }"
        :style="nameStyle(author)"
        @click="emit('profile', message.senderId)"
        @contextmenu.stop="authorMenu"
      >
        {{ author?.nick ?? author?.displayName }}
      </button>
      <div class="message-body min-w-0">
        <em
          v-if="message.deleted"
          class="message-text message-deleted font-chat leading-[1.5] wrap-anywhere whitespace-pre-wrap text-muted-foreground italic"
          >Message deleted</em
        >
        <!-- A div, not a span: code blocks put a <pre> in here. -->
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText"
          class="message-text font-chat leading-[1.5] text-body wrap-anywhere whitespace-pre-wrap"
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
      </div>
    </div>

    <div
      v-else
      class="message-grid grid [grid-template-columns:40px_minmax(0,_1fr)] gap-x-3.5"
    >
      <button
        v-if="!grouped"
        class="avatar-button [align-self:start] mt-0.5 rounded-full flex active:translate-y-px [&_.avatar]:size-10 [&_.avatar]:text-[16px]"
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
        class="grouped-time text-[11px] text-faint leading-[calc(var(--message-size)_*_1.5)] text-right opacity-0 tabular-nums whitespace-nowrap group-hover/message:opacity-100 [@media(hover:none)]:opacity-0"
        :datetime="message.createdAt"
        :title="full(message.createdAt)"
        >{{ time(message.createdAt) }}</time
      >
      <!-- Keep grouped messages in the text column even when compact mode hides the time. -->
      <div class="message-body min-w-0 col-start-2">
        <div
          v-if="!grouped"
          class="message-meta flex items-baseline gap-2 min-w-0 [@media(hover:none)_and_(max-width:700px)]:pr-7"
        >
          <button
            class="author-name text-[15px] font-[600] leading-[1.375] overflow-hidden text-ellipsis whitespace-nowrap max-w-full hover:underline"
            :style="nameStyle(author)"
            @click="emit('profile', message.senderId)"
            @contextmenu.stop="authorMenu"
          >
            {{ author?.displayName }}</button
          ><span
            v-if="author?.role === 'bot'"
            class="bot-badge self-center inline-flex items-center h-4 py-0 px-[5px] rounded-[4px] bg-selection text-primary text-[10px] font-[650] tracking-[0.04em] shrink-0"
            >BOT</span
          ><time
            class="text-[12px] text-faint tabular-nums whitespace-nowrap"
            :datetime="message.createdAt"
            :title="full(message.createdAt)"
            >{{ time(message.createdAt) }}</time
          >
        </div>
        <p
          v-if="message.deleted"
          class="message-text message-deleted font-chat leading-[1.5] wrap-anywhere whitespace-pre-wrap text-muted-foreground italic"
        >
          <em class="[font-style:inherit]">Message deleted</em>
        </p>
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText"
          class="message-text font-chat leading-[1.5] text-body wrap-anywhere whitespace-pre-wrap"
        >
          <span
            v-if="action"
            class="action-nick font-[600] not-italic mr-[0.3em]"
            :style="nameStyle(author)"
            >{{ author?.nick }}
          </span>
          <MessageContent
            :text="message.text"
            code-blocks
            :continues-line="action"
          /><MessageEdited
            v-if="message.editedAt"
            v-model:open="showEdits"
            :message="message"
          />
        </div>
        <MessageExtras
          v-if="!message.deleted"
          :message="message"
        />
      </div>
    </div>

    <MessageExtras
      v-if="irc && !message.deleted"
      :message="message"
    />

    <ol
      v-if="showEdits && previousVersions.length && !message.deleted"
      class="message-edit-history mt-1 mr-0 mb-1 ml-13.5 py-1.5 px-2.5 list-none border-l-2 [border-left-style:solid] border-l-[var(--line)] text-[0.85em] text-muted-foreground [&>li+li]:mt-1"
      aria-label="Earlier versions"
    >
      <li
        v-for="(version, i) in previousVersions"
        :key="i"
      >
        <time
          class="block text-[0.85em]"
          :datetime="version.at"
          :title="full(version.at)"
          >{{ i === 0 ? 'Original' : 'Edited' }} {{ time(version.at) }}</time
        >
        <MessageContent :text="version.text" />
      </li>
    </ol>
    <UiButton
      variant="ghost"
      size="icon-sm"
      v-if="!message.delivery"
      class="mobile-message-menu hidden absolute right-1 top-0 text-faint [@media(hover:none)]:flex"
      aria-label="Message actions"
      aria-haspopup="menu"
      @click="messageMenu($event.currentTarget as HTMLElement)"
    >
      <AppIcon
        name="more"
        :size="16"
      />
    </UiButton>
    <div
      v-if="!message.delivery && !message.deleted"
      class="message-actions absolute right-3 top-[-16px] flex items-center p-0.5 bg-background rounded-md shadow-[var(--shadow-small),_0_0_0_1px_var(--border)] opacity-0 pointer-events-none z-2 [&_.icon-button]:w-7.5 [&_.icon-button]:h-7 [&_.icon-button:disabled]:opacity-40 [&_.icon-button:disabled]:cursor-not-allowed [&_.danger:hover:not(:disabled)]:text-destructive group-hover/message:opacity-100 group-hover/message:pointer-events-auto group-focus-within/message:opacity-100 group-focus-within/message:pointer-events-auto group-[.show-actions]/message:opacity-100 group-[.show-actions]/message:pointer-events-auto [@media(hover:none)]:hidden"
      role="toolbar"
      :aria-label="alternate ? 'More message actions' : 'Message actions'"
    >
      <template v-if="!alternate">
        <template v-if="reactionSupported">
          <UiButton
            variant="ghost"
            size="icon-sm"
            v-for="item in quickReactions"
            :key="item.id"
            class="quick-reaction text-[16px]"
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
          </UiButton>
          <span class="action-divider w-[1px] h-4.5 my-0 mx-0.5 bg-border" />
        </template>
        <UiButton
          variant="ghost"
          size="icon-sm"
          v-if="reactionSupported"
          :ref="(el) => (reactButton = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
          aria-label="Add reaction"
          :disabled="!!reactionReason"
          :title="reactionReason ?? 'Add reaction'"
          :aria-expanded="reacting"
          @click="reacting ? (reacting = false) : openReactions(reactButton)"
        >
          <AppIcon
            name="smilePlus"
            :size="17" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          v-if="replySupported"
          aria-label="Reply to message"
          :disabled="!!replyReason"
          :title="replyReason ?? 'Reply'"
          @click="emit('reply', message)"
        >
          <AppIcon
            name="reply"
            :size="17" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          v-if="editable"
          aria-label="Edit message"
          :disabled="!editable.allowed || editing"
          :title="editable.why ?? 'Edit'"
          @click="messageEditing.start(message)"
        >
          <AppIcon
            name="pencil"
            :size="16" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          aria-label="Copy message"
          title="Copy text"
          @click="menus.copyMessage(message)"
        >
          <AppIcon
            name="copy"
            :size="16" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          :class="{
            'is-saved text-primary': isSaved,
            'is-saving opacity-55 cursor-progress': !!saveChange,
          }"
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
        </UiButton>
      </template>
      <template v-else>
        <UiButton
          variant="ghost"
          size="icon-sm"
          v-if="unreadable"
          aria-label="Mark as unread"
          title="Mark as unread"
          @click="chat.markUnread(message)"
        >
          <AppIcon
            name="unread"
            :size="16" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          v-if="removal"
          class="danger"
          aria-label="Delete message"
          :title="removal.why ?? 'Delete for everyone'"
          :disabled="!removal.allowed"
          @click="menus.deleteMessage(message)"
        >
          <AppIcon
            name="trash"
            :size="16"
          />
        </UiButton>
      </template>
      <UiButton
        variant="ghost"
        size="icon-sm"
        :ref="(el) => (moreButton = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
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
      </UiButton>
    </div>
    <ExpressionPicker
      v-if="reacting && reactAnchor"
      :anchor="reactAnchor"
      tab="emoji"
      emoji-only
      @close="reacting = false"
      @emoji="react"
    />
  </article>
</template>
