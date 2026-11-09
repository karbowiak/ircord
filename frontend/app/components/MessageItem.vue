<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import { stripFormatting } from '../../shared/utils/irc'
import { attractClock } from '../../shared/utils/attract'
import {
  attachmentMediaAsset,
  messageMediaLinks,
  messageMediaText,
  messageAttachments,
  publicMediaURL,
} from '../../shared/utils/link-media'
import { selectEmbed } from '../utils/embed'
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
const { t, locale } = useI18n()
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
/** An @mention of your nick on this network, a highlight word, or a group mention including you. */
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
/** URL presentation is known before paint; stored text still drives copy, export and editing. */
const presentedLinks = computed(() => {
  const message = props.message
  const attachments = messageAttachments(message)
  if (message.deleted) return new Set<string>()
  const uploadURLs = new Set(
    attachments
      .map((attachment) => attachmentMediaAsset(attachment, window.location.href)?.url)
      .filter(Boolean),
  )
  return new Set(
    messageMediaLinks(message.text).filter((url) =>
      attachments.length ? uploadURLs.has(publicMediaURL(url)) : selectEmbed(url).provider.hideUrl,
    ),
  )
})
const presentedText = computed(() => messageMediaText(props.message.text, presentedLinks.value))
const showText = computed(() => !!stripFormatting(presentedText.value).trim())
/** The three most recently used emoji (defaults until there is history). */
const quickReactions = computed(() => library.quickReactions.slice(0, 3))
const menus = useMenus()
const saved = useSavedStore()
const isSaved = computed(() => saved.isSaved(props.message.id))
const saveChange = computed(() => saved.pendingChange(props.message.id))
const saveLabel = computed(() =>
  saveChange.value === 'saving'
    ? t('common.saving')
    : saveChange.value === 'removing'
      ? t('workspaceUi.message.removingSaved')
      : isSaved.value
        ? t('workspaceUi.message.removeSaved')
        : t('workspaceUi.message.saveLater'),
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
  const hidden = [
    unreadable.value && t('workspaceUi.message.markUnread'),
    removal.value && t('workspaceUi.message.delete'),
  ].filter((value): value is string => !!value)
  return alternate.value || !hidden.length
    ? t('workspaceUi.message.moreActions')
    : t('workspaceUi.message.moreActionsHint', {
        actions: new Intl.ListFormat(locale.value, { type: 'conjunction' }).format(hidden),
      })
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

// The toolbar is one Tab stop; arrow keys, Home and End move between its buttons (the ARIA
// toolbar pattern). Buttons come and go with Shift and capabilities, so the roving tabindex is
// reapplied after each render. Disabled buttons are skipped, as the browser skips them anyway.
const toolbar = ref<HTMLElement | null>(null)
const toolIndex = ref(0)
const tools = () =>
  [...(toolbar.value?.querySelectorAll<HTMLButtonElement>('button') ?? [])].filter(
    (button) => !button.disabled,
  )
function rove() {
  const list = tools()
  const current = list[Math.min(toolIndex.value, list.length - 1)]
  for (const button of list) button.tabIndex = button === current ? 0 : -1
}
onMounted(rove)
onUpdated(rove)
function toolbarFocus(event: FocusEvent) {
  const index = tools().indexOf(event.target as HTMLButtonElement)
  if (index >= 0) toolIndex.value = index
  rove()
}
function toolbarKeys(event: KeyboardEvent) {
  const list = tools()
  if (!list.length || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const index = list.indexOf(document.activeElement as HTMLButtonElement)
  toolIndex.value =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? list.length - 1
        : (index + (event.key === 'ArrowRight' ? 1 : -1) + list.length) % list.length
  rove()
  list[toolIndex.value]?.focus()
}
/** The article's name: author and time, plus states the row shows only as tint or opacity. */
const rowLabel = computed(() =>
  [
    author.value?.displayName ?? author.value?.nick ?? t('workspaceUi.message.unknownSender'),
    time(props.message.createdAt),
    mentionsMe.value && t('workspaceUi.message.mentionsYou'),
    props.message.delivery === 'pending' && t('workspaceUi.message.sending'),
  ]
    .filter(Boolean)
    .join(', '),
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
    ref="row"
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
    :aria-label="rowLabel"
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
          >{{ $t('workspaceUi.message.deleted') }}</em
        >
        <!-- A div, not a span: code blocks put a <pre> in here. -->
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText || message.editedAt"
          class="message-text font-chat leading-[1.5] text-body wrap-anywhere whitespace-pre-wrap"
        >
          <MessageContent
            :text="presentedText"
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
        :aria-label="
          $t('workspaceUi.message.viewProfile', {
            name: author?.displayName ?? author?.nick ?? $t('workspaceUi.message.thisPerson'),
          })
        "
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
        class="grouped-time text-[11px] text-faint leading-[calc(var(--message-size)_*_1.5)] text-right opacity-0 tabular-nums whitespace-nowrap group-hover/message:opacity-100 group-focus-within/message:opacity-100 [@media(hover:none)]:opacity-0"
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
            class="bot-badge self-center inline-flex items-center h-4 py-0 px-[5px] rounded-[4px] bg-selection text-primary text-[10px] font-[650] tracking-[0.04em] uppercase shrink-0"
            >{{ $t('collectionsUi.search.bot') }}</span
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
          <em class="[font-style:inherit]">{{ $t('workspaceUi.message.deleted') }}</em>
        </p>
        <MessageEditor
          v-else-if="editing"
          :message="message"
        />
        <div
          v-else-if="showText || action || message.editedAt"
          class="message-text font-chat leading-[1.5] text-body wrap-anywhere whitespace-pre-wrap"
        >
          <span
            v-if="action"
            class="action-nick font-[600] not-italic mr-[0.3em]"
            :style="nameStyle(author)"
            >{{ author?.nick }}
          </span>
          <MessageContent
            :text="presentedText"
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
      :aria-label="$t('workspaceUi.message.earlierVersions')"
    >
      <li
        v-for="(version, i) in previousVersions"
        :key="i"
      >
        <time
          class="block text-[0.85em]"
          :datetime="version.at"
          :title="full(version.at)"
          >{{
            $t(i === 0 ? 'workspaceUi.message.originalAt' : 'workspaceUi.message.editedAt', {
              time: time(version.at),
            })
          }}</time
        >
        <MessageContent :text="version.text" />
      </li>
    </ol>
    <UiButton
      v-if="!message.delivery"
      variant="ghost"
      size="icon-sm"
      class="mobile-message-menu hidden absolute right-1 top-0 text-faint [@media(hover:none)]:flex"
      :aria-label="$t('workspaceUi.message.actions')"
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
      ref="toolbar"
      class="message-actions absolute right-3 top-[-16px] flex items-center p-0.5 bg-background rounded-md shadow-[var(--shadow-small),_0_0_0_1px_var(--border)] opacity-0 pointer-events-none z-2 [&_.icon-button]:w-7.5 [&_.icon-button]:h-7 [&_.icon-button:disabled]:opacity-40 [&_.icon-button:disabled]:cursor-not-allowed [&_.danger:hover:not(:disabled)]:text-destructive group-hover/message:opacity-100 group-hover/message:pointer-events-auto group-focus-within/message:opacity-100 group-focus-within/message:pointer-events-auto group-[.show-actions]/message:opacity-100 group-[.show-actions]/message:pointer-events-auto [@media(hover:none)]:hidden"
      role="toolbar"
      :aria-label="
        alternate ? $t('workspaceUi.message.moreMessageActions') : $t('workspaceUi.message.actions')
      "
      @focusin="toolbarFocus"
      @keydown="toolbarKeys"
    >
      <template v-if="!alternate">
        <template v-if="reactionSupported">
          <UiButton
            v-for="item in quickReactions"
            :key="item.id"
            variant="ghost"
            size="icon-sm"
            class="quick-reaction text-[16px]"
            :aria-label="$t('workspaceUi.message.reactWith', { emoji: emojiLabel(item) })"
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
          v-if="reactionSupported"
          :ref="(el) => (reactButton = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('workspaceUi.message.addReaction')"
          :disabled="!!reactionReason"
          :title="reactionReason ?? $t('workspaceUi.message.addReaction')"
          :aria-expanded="reacting"
          @click="reacting ? (reacting = false) : openReactions(reactButton)"
        >
          <AppIcon
            name="smilePlus"
            :size="17" /></UiButton
        ><UiButton
          v-if="replySupported"
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('workspaceUi.message.replyTo')"
          :disabled="!!replyReason"
          :title="replyReason ?? $t('workspaceUi.message.reply')"
          @click="emit('reply', message)"
        >
          <AppIcon
            name="reply"
            :size="17" /></UiButton
        ><UiButton
          v-if="editable"
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('workspaceUi.message.edit')"
          :disabled="!editable.allowed || editing"
          :title="editable.why ?? $t('common.edit')"
          @click="messageEditing.start(message)"
        >
          <AppIcon
            name="pencil"
            :size="16" /></UiButton
        ><UiButton
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('workspaceUi.message.copy')"
          :title="$t('workspaceUi.message.copyText')"
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
          v-if="unreadable"
          variant="ghost"
          size="icon-sm"
          :aria-label="$t('workspaceUi.message.markUnread')"
          :title="$t('workspaceUi.message.markUnread')"
          @click="chat.markUnread(message)"
        >
          <AppIcon
            name="unread"
            :size="16" /></UiButton
        ><UiButton
          v-if="removal"
          variant="ghost"
          size="icon-sm"
          class="danger"
          :aria-label="$t('workspaceUi.message.deleteMessage')"
          :title="removal.why ?? $t('workspaceUi.message.deleteEveryone')"
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
        :ref="(el) => (moreButton = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
        variant="ghost"
        size="icon-sm"
        :aria-label="$t('workspaceUi.message.moreActions')"
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
