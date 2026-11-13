<script setup vapor lang="ts">
import { useQuery } from '@pinia/colada'
import { useEventListener, useResizeObserver } from '@vueuse/core'
import type { Message } from '../../../shared/types/chat'
import { messagesQuery } from '../../queries/chat'
import type { EmojiItem, GifItem } from '../../../shared/types/media'
import { emojiText } from '../../../shared/utils/media'
import { stripFormatting } from '../../../shared/utils/irc'
import { clipboardListText, normalizeBullets } from '../../../shared/utils/lists'
import { shortcutOf } from '../../../shared/utils/shortcuts'
import { overlayOpen } from '../../../shared/utils/focus'
import { formatBytes } from '../../../shared/utils/bytes'
import { useMessageActions } from '../../composables/chat/useMessageActions'
import { partyName } from '../../../shared/utils/party'
const props = defineProps<{ replyTo?: Message }>()
const emit = defineEmits<{ cancelReply: [] }>()
const { t, locale, n: $n } = useI18n()
const $t = t
const chat = useChatStore()
const auth = useAuthStore()
const library = useLibraryStore()
const media = useMediaStore()
const uploads = useUploads()
const { run, gifLookup } = useCommands()
const { open: openMenu } = useContextMenu()
const nameStyle = useNameStyle()
const messageActions = useMessageActions()
const messageEditing = useMessageEditing()
const history = useQuery(() => messagesQuery(chat.activeId))
const input = ref<HTMLTextAreaElement | null>(null)
useTypeToCompose(input)
const fileInput = ref<HTMLInputElement | null>(null)
const composerElement = ref<HTMLFormElement | null>(null)
const gifTrigger = ref<HTMLButtonElement | null>(null)
const emojiTrigger = ref<HTMLButtonElement | null>(null)
const pickerTab = ref<'gifs' | 'emoji' | null>(null)
const dialogs = useDialogStack()
let unregisterPicker: (() => void) | undefined
watch(
  () => !!pickerTab.value,
  (open) => {
    if (open) unregisterPicker = dialogs.register(() => closePicker(true))
    else {
      unregisterPicker?.()
      unregisterPicker = undefined
    }
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => unregisterPicker?.())
useViewHistory('composer-picker', pickerTab)
/**
 * Where focus goes when the picker is dismissed: its trigger button when it was clicked open,
 * the message box (with its caret) when it was opened from the keyboard.
 */
let pickerReturn:
  'trigger' | { start: number; end: number; direction: HTMLTextAreaElement['selectionDirection'] } =
  'trigger'
const shortcutKeys = useShortcuts()
const gifHint = computed(() => shortcutKeys.label('gifs'))
const inputHelp = useId()
const suggestionsId = useId()

const draft = computed({
  get: () => chat.drafts[chat.activeId] ?? '',
  set: (value) => {
    if (value) chat.drafts[chat.activeId] = value
    else delete chat.drafts[chat.activeId]
  },
})
/** In the server window, plain text is sent to the server as a raw line. */
const rawMode = computed(() => !!chat.serverView)
const enabled = computed(() => chat.canSend)
const target = computed(() =>
  rawMode.value
    ? (chat.network?.name ?? t('workspaceUi.composer.server'))
    : chat.active?.party
      ? partyName(chat.active.party, auth.account?.id, t('workspaceUi.party.unnamed'))
      : chat.active?.kind === 'channel'
        ? `#${chat.active.name}`
        : `@${chat.active ? (chat.directPartner(chat.active)?.nick ?? chat.active.name) : ''}`,
)
const placeholder = computed(() =>
  !enabled.value
    ? chat.status !== 'connected' || !chat.network
      ? t('workspaceUi.composer.waiting')
      : chat.network.status === 'connecting'
        ? t('workspaceUi.composer.connecting', { network: chat.network.name })
        : chat.network.status === 'error'
          ? t('workspaceUi.composer.networkError', {
              network: chat.network.name,
              detail: chat.network.statusDetail ?? t('workspaceUi.composer.connectionFailed'),
            })
          : (chat.sendReason ??
            t('workspaceUi.composer.disconnected', { network: chat.network.name }))
    : rawMode.value
      ? t('workspaceUi.composer.commandPlaceholder', { target: target.value })
      : t('workspaceUi.composer.messageTarget', { target: target.value }),
)

/** The query of a /gif lookup running for the open conversation; shown until it finishes. */
const gifQuery = computed(() => gifLookup(chat.activeId))
useAnnouncedStatus(
  () => (gifQuery.value ? t('workspaceUi.composer.findingGif', { query: gifQuery.value }) : ''),
  {
    context: () => chat.activeId,
    delay: 300,
  },
)
const typingText = computed(() => {
  const nicks = (chat.typing[chat.activeId] ?? []).map((id) => chat.member(id)?.nick ?? id)
  if (!nicks.length) return ''
  if (nicks.length === 1) return t('workspaceUi.composer.typingOne', { names: nicks[0] })
  if (nicks.length <= 3)
    return t('workspaceUi.composer.typingMany', {
      names: new Intl.ListFormat(locale.value, { type: 'conjunction' }).format(nicks),
    })
  return t('workspaceUi.composer.typingSeveral')
})
const auto = useComposerAutocomplete({ draft, input, rawMode, setDraft })
const { token, index: suggestionIndex, count: suggestionCount, complete } = auto
const { emoji: emojiSuggestions, members: memberSuggestions } = auto.suggestions
const groupSuggestions = auto.suggestions.groups
const { commands: commandSuggestions, channels: channelSuggestions } = auto.suggestions
function setDraft(value: string, position: number) {
  draft.value = value
  nextTick(() => {
    input.value?.focus()
    input.value?.setSelectionRange(position, position)
    auto.caret.value = position
    resize()
  })
}

// Reaching the composer from the keyboard (Tab, the skip link, a shortcut) shows a neutral ring,
// never the accent. Clicking into it, or typing once there, keeps the plain frame.
const keyboardFocus = ref(false)
let lastInput: 'keyboard' | 'pointer' = 'pointer'
useEventListener(document, 'keydown', () => (lastInput = 'keyboard'), { capture: true })
useEventListener(document, 'pointerdown', () => (lastInput = 'pointer'), { capture: true })
function onFocus() {
  media.needAll()
  keyboardFocus.value = lastInput === 'keyboard'
}
function onKeydown(event: KeyboardEvent) {
  keyboardFocus.value = false
  keydown(event)
}
function onBlur() {
  keyboardFocus.value = false
  auto.dismiss()
}
function onInput() {
  keyboardFocus.value = false
  auto.onInput()
  if (!rawMode.value && draft.value.trim() && !draft.value.startsWith('/'))
    chat.sendTyping('active')
  else chat.sendTyping('done')
}
function resize() {
  if (input.value) {
    if (!input.value.value) {
      input.value.style.height = ''
      return
    }
    input.value.style.height = 'auto'
    input.value.style.height = `${Math.min(input.value.scrollHeight, 200)}px`
  }
}
// Draft changes can reveal Send and narrow the field. Rotation and unfolding also change
// wrapping without an input event, so measure after the controls and field have reflowed.
watch(draft, () => nextTick(resize), { immediate: true })
let inputWidth = 0
let resizeFrame = 0
useResizeObserver(input, ([entry]) => {
  if (!entry || entry.contentRect.width === inputWidth) return
  inputWidth = entry.contentRect.width
  cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(resize)
})
onBeforeUnmount(() => cancelAnimationFrame(resizeFrame))

// ---- Sending ----------------------------------------------------------------------------------

async function send() {
  if (!enabled.value) return
  const text = draft.value
  const files = uploads.pending.value.length
  if (!text.trim() && !files) return
  if (text.length > 4000) return chat.notify(t('workspaceUi.composer.tooLong'))
  if (text.trim().startsWith('/')) {
    const origin = chat.activeId
    const reply = props.replyTo
    const result = await run(text, { replyTo: reply })
    if (result === 'kept') return
    if (result === 'handled') return spend(origin, text, reply)
  }
  if (rawMode.value && text.trim()) {
    if (chat.raw(text.trim())) clear()
    return
  }
  // “//text” sends “/text” literally.
  const message = text.trim().startsWith('//') ? text.trim().slice(1) : text
  const reply = props.replyTo
  if (message.trim() && !chat.send(message, props.replyTo)) return
  clear()
  if (files) void uploads.sendAll(message.trim() ? undefined : reply)
}
function clear() {
  emit('cancelReply')
  clearInput()
}
function clearInput() {
  chat.sendTyping('done')
  draft.value = ''
  nextTick(resize)
  input.value?.focus()
}
/**
 * Spends a finished command. Commands can await (a /gif lookup takes seconds), and meanwhile the
 * user may type, reply or open another conversation: only the exact text that was submitted is
 * cleared, only from the conversation it was typed in, and only the reply it used is cancelled.
 */
function spend(origin: string, text: string, reply: Message | undefined) {
  if (chat.activeId !== origin) {
    // Switching conversations already reset the reply bar.
    if (chat.drafts[origin] === text) delete chat.drafts[origin]
    return
  }
  if (props.replyTo === reply) emit('cancelReply')
  if (draft.value === text) clearInput()
}

/** Up in an empty composer edits your newest editable message here, as in Discord. */
function editLatest(event: KeyboardEvent) {
  if (event.key !== 'ArrowUp' || draft.value || event.isComposing) return false
  if (event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return false
  const latest = (history.data.value?.items ?? []).findLast(
    (m) =>
      m.senderId === 'you' && !m.deleted && !m.delivery && !!messageActions.editing(m)?.allowed,
  )
  if (!latest) return false
  event.preventDefault()
  messageEditing.start(latest)
  return true
}
function keydown(event: KeyboardEvent) {
  if (auto.keydown(event)) return
  if (editLatest(event)) return
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    void send()
  }
  const format = shortcutOf(event, ['bold', 'italic', 'underline'], shortcutKeys.bindings.value)
  if (format && !rawMode.value && !event.isComposing) {
    event.preventDefault()
    wrap({ bold: '**', italic: '*', underline: '__' }[format])
  }
  if (event.key === 'Escape') {
    pickerTab.value = null
    emit('cancelReply')
  }
}
/** Wraps the selection in a formatting marker; it becomes an IRC code when sent. */
function wrap(marker: string) {
  const start = input.value?.selectionStart ?? draft.value.length
  const end = input.value?.selectionEnd ?? start
  const selected = draft.value.slice(start, end)
  draft.value = `${draft.value.slice(0, start)}${marker}${selected}${marker}${draft.value.slice(end)}`
  nextTick(() => {
    input.value?.focus()
    input.value?.setSelectionRange(start + marker.length, start + marker.length + selected.length)
    resize()
  })
}
function insertEmoji(item: EmojiItem) {
  const value = emojiText(item, library.library.skinTone)
  const start = input.value?.selectionStart ?? draft.value.length
  const end = input.value?.selectionEnd ?? start
  if (draft.value.length - (end - start) + value.length > 4000) {
    chat.notify(t('workspaceUi.composer.characterLimit'))
    return
  }
  pickerTab.value = null
  setDraft(draft.value.slice(0, start) + value + draft.value.slice(end), start + value.length)
}
function sendGif(item: GifItem, query: string) {
  if (chat.send(item.url, props.replyTo, item)) {
    pickerTab.value = null
    emit('cancelReply')
    klipyShare(media.appKey, item, query)
    input.value?.focus()
  }
}
function togglePicker(tab: 'gifs' | 'emoji') {
  pickerReturn = 'trigger'
  pickerTab.value = pickerTab.value === tab ? null : tab
}
function closePicker(restoreFocus: boolean) {
  const last = pickerTab.value
  pickerTab.value = null
  if (!restoreFocus) return
  const back = pickerReturn
  nextTick(() => {
    if (back === 'trigger')
      return (last === 'gifs' ? gifTrigger.value : emojiTrigger.value)?.focus()
    input.value?.focus()
    input.value?.setSelectionRange(back.start, back.end, back.direction)
  })
}
/** Ctrl/⌘ G: opens the GIF picker on its search, or closes it again. */
function gifShortcut() {
  if (pickerTab.value === 'gifs') {
    closePicker(true)
    return true
  }
  if (pickerTab.value === 'emoji') {
    pickerTab.value = 'gifs'
    nextTick(() => document.querySelector<HTMLElement>('.expression-search input')?.focus())
    return true
  }
  if (rawMode.value || !enabled.value || !input.value || overlayOpen()) return false
  const caret = {
    start: input.value.selectionStart,
    end: input.value.selectionEnd,
    direction: input.value.selectionDirection,
  }
  pickerReturn =
    document.activeElement === input.value
      ? caret
      : { start: draft.value.length, end: draft.value.length, direction: 'none' }
  pickerTab.value = 'gifs'
  return true
}
/** The focus-the-message-box shortcut; already there, the key types as usual. */
function focusShortcut() {
  const field = input.value
  if (!field || field.disabled || document.activeElement === field) return false
  if (!field.isConnected || field.closest('[inert]')) return false
  field.focus()
  return true
}
const actions = useWorkspaceActions()
actions.gifs = gifShortcut
actions.focusComposer = focusShortcut
onBeforeUnmount(() => {
  if (actions.gifs === gifShortcut) actions.gifs = () => false
  if (actions.focusComposer === focusShortcut) actions.focusComposer = () => false
})

// ---- Attachments ------------------------------------------------------------------------------

function plusMenu(event: MouseEvent) {
  openMenu(event.currentTarget as HTMLElement, [
    {
      label: t('workspaceUi.composer.upload'),
      icon: 'upload',
      disabled: !uploads.capability.value,
      hint: uploads.capability.value
        ? formatBytes(uploads.capability.value.maxBytes, locale.value)
        : t('workspaceUi.composer.unavailable'),
      action: () => fileInput.value?.click(),
    },
    {
      label: t('workspaceUi.composer.sendGif'),
      icon: 'gif',
      hint: gifHint.value || undefined,
      action: () => togglePicker('gifs'),
    },
    {
      label: t('workspaceUi.composer.command'),
      icon: 'slash',
      action: () => setDraft('/', 1),
    },
  ])
}
function chooseFiles(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files?.length) void uploads.add(target.files)
  target.value = ''
  input.value?.focus()
}
function paste(event: ClipboardEvent) {
  if (rawMode.value) return
  const files = [...(event.clipboardData?.files ?? [])]
  if (files.length) {
    event.preventDefault()
    void uploads.add(files)
    return
  }
  const text = pastedList(event.clipboardData)
  if (text === null) return
  event.preventDefault()
  insertText(text)
}
/**
 * A list copied from a web page or document pastes as Markdown list lines, which render as a list
 * once sent; plain text with bullet glyphs (•) gets the same markers. Null keeps the normal paste.
 */
function pastedList(data: DataTransfer | null) {
  const lines = (text: string) => text.replace(/\r\n?/g, '\n')
  const html = data?.getData('text/html') ?? ''
  if (/<li[\s>]/i.test(html)) {
    const text = clipboardListText(new DOMParser().parseFromString(html, 'text/html').body)
    if (text) return lines(text)
  }
  const plain = lines(data?.getData('text/plain') ?? '')
  const bullets = normalizeBullets(plain)
  return bullets !== plain ? bullets : null
}
/** Replaces the selection as typing would, so the browser's undo still works. */
function insertText(text: string) {
  const box = input.value
  if (!box) return
  box.focus()
  if (document.execCommand('insertText', false, text)) return
  const start = box.selectionStart
  const end = box.selectionEnd
  setDraft(draft.value.slice(0, start) + text + draft.value.slice(end), start + text.length)
}

watch(
  () => chat.activeId,
  () => {
    pickerTab.value = null
    auto.reset()
    nextTick(resize)
  },
)
watch(
  () => props.replyTo,
  () => input.value?.focus(),
)
// Menus ask for text such as “@nick ” to be inserted at the caret.
watch(
  () => chat.pendingInsert,
  (text) => {
    if (!text) return
    chat.pendingInsert = ''
    const start = input.value?.selectionStart ?? draft.value.length
    const spacer = start > 0 && !/\s$/.test(draft.value.slice(0, start)) ? ' ' : ''
    setDraft(
      draft.value.slice(0, start) + spacer + text + draft.value.slice(start),
      start + spacer.length + text.length,
    )
  },
)
defineExpose({
  focus: () => input.value?.focus(),
  /** Focuses the message box with the caret after its draft (opened from the Drafts list). */
  /** Returns false when the box is unavailable, so the caller can focus elsewhere. */
  resume: () => {
    if (!input.value || input.value.disabled) return false
    const end = draft.value.length
    input.value.focus()
    input.value.setSelectionRange(end, end)
    auto.caret.value = end
    resize()
    return true
  },
})
</script>
<template>
  <div
    class="composer-area pt-0 pr-4 pl-4 shrink-0 pb-[max(12px,env(safe-area-inset-bottom))] relative [&>.composer-reply+.composer]:rounded-t-none max-[700px]:px-1.5 max-[700px]:pb-[max(6px,env(safe-area-inset-bottom))] max-[700px]:[&_.typing-line]:left-2.5"
  >
    <div
      v-if="replyTo"
      class="composer-reply flex items-center gap-2 bg-card pt-1.5 pr-2 pb-1.5 pl-3.5 rounded-[var(--radius)_var(--radius)_0_0] text-muted-foreground text-[13px] shadow-[inset_0_0_0_1px_var(--border)]"
    >
      <AppIcon
        name="reply"
        :size="15"
        class="text-primary"
      />
      <i18n-t
        keypath="workspaceUi.composer.replying"
        tag="span"
      >
        <template #name
          ><strong :style="nameStyle(chat.member(replyTo.senderId))">{{
            chat.member(replyTo.senderId)?.displayName
          }}</strong></template
        >
      </i18n-t>
      <span
        dir="auto"
        class="reply-excerpt flex-1 text-faint overflow-hidden whitespace-nowrap text-ellipsis min-w-0"
        >{{ stripFormatting(replyTo.text) }}</span
      >
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="size-6 ml-auto"
        :aria-label="$t('workspaceUi.composer.cancelReply')"
        @click="emit('cancelReply')"
      >
        <AppIcon
          name="close"
          :size="16"
        />
      </UiButton>
    </div>
    <ComposerSuggestions
      v-if="token && suggestionCount"
      :kind="token.kind"
      :emoji="emojiSuggestions"
      :members="memberSuggestions"
      :groups="groupSuggestions"
      :commands="commandSuggestions"
      :channels="channelSuggestions"
      :active="suggestionIndex"
      :query="token.query"
      :list-id="suggestionsId"
      @emoji="complete.emoji"
      @member="complete.member"
      @group="complete.group"
      @command="complete.command"
      @channel="complete.channel"
      @hover="suggestionIndex = $event"
    />
    <ComposerUploads />
    <form
      ref="composerElement"
      class="composer flex items-end gap-1 min-h-12 pt-[9px] pr-1.5 pb-[9px] bg-field rounded-md shadow-[inset_0_0_0_1px_var(--border)] [transition:box-shadow_0.15s] pl-1.5 [&.composer-disabled]:opacity-70 [&.is-raw]:pl-3.5 [&.is-raw]:font-code [&_.icon-button]:w-[34px] [&_.icon-button]:h-7.5 [&_.icon-button]:text-foreground [&_.icon-button]:opacity-[0.78] [&_.icon-button:hover:not(:disabled)]:opacity-100 [&_.icon-button.is-open]:opacity-100 [&_.icon-button.is-open]:bg-accent max-[700px]:min-h-10 max-[700px]:py-0.5 max-[700px]:[&.has-draft:has(textarea:focus)_.composer-expression]:hidden"
      :class="{
        'composer-disabled': !enabled,
        'is-raw': rawMode,
        'is-keyboard-focus': keyboardFocus,
        'has-draft': !!draft.length,
      }"
      :aria-busy="!!gifQuery || undefined"
      @submit.prevent="send"
    >
      <AppTooltip
        v-if="!rawMode"
        :text="$t('workspaceUi.composer.attachHelp')"
        side="top"
      >
        <UiButton
          variant="ghost"
          size="icon-sm"
          type="button"
          class="composer-plus mr-0.5"
          :aria-label="$t('workspaceUi.composer.attach')"
          :disabled="!enabled"
          @click="plusMenu"
        >
          <AppIcon
            name="plus"
            :size="20"
          />
        </UiButton>
      </AppTooltip>
      <span
        v-else
        class="raw-prompt self-center text-body font-code text-[12px] mr-1.5"
        aria-hidden="true"
        >/quote</span
      >
      <input
        ref="fileInput"
        type="file"
        class="sr-only"
        multiple
        :accept="uploads.capability.value?.types.join(',')"
        tabindex="-1"
        aria-hidden="true"
        @change="chooseFiles"
      />
      <!-- Drafting stays available while sending reconnects. Disabling the same focused
           textarea node blurs it before the network comes back. -->
      <textarea
        ref="input"
        v-model="draft"
        dir="auto"
        class="block flex-1 self-center min-w-0 h-7.5 min-h-7.5 max-h-50 m-0 resize-none py-1 px-0 bg-transparent border-0 text-foreground text-[length:var(--message-size)] leading-[22px] outline-none! placeholder:text-faint max-[700px]:text-[16px] [@media(pointer:coarse)]:text-[length:max(16px,var(--message-size))]"
        :class="rawMode ? 'font-code' : 'font-chat'"
        data-saved-draft
        :aria-label="
          rawMode
            ? $t('workspaceUi.composer.commandTarget', { target })
            : $t('workspaceUi.composer.messageTarget', { target })
        "
        :aria-describedby="inputHelp"
        :aria-keyshortcuts="rawMode ? undefined : shortcutKeys.aria('gifs')"
        :placeholder="placeholder"
        rows="1"
        :maxlength="4000"
        aria-autocomplete="list"
        :aria-controls="suggestionCount ? suggestionsId : undefined"
        :aria-activedescendant="suggestionCount ? `${suggestionsId}-${suggestionIndex}` : undefined"
        @input="onInput"
        @keydown="onKeydown"
        @keyup="auto.trackCaret"
        @click="auto.trackCaret"
        @focus="onFocus"
        @blur="onBlur"
        @paste="paste"
      />
      <div
        v-if="!rawMode"
        class="composer-actions flex shrink-0 items-center gap-0.5 min-h-7.5"
      >
        <span
          v-if="draft.length > 3600"
          class="character-count text-[12px] text-warning py-0 px-1 tabular-nums"
          >{{ $n(4000 - draft.length) }}</span
        >
        <AppTooltip
          :text="shortcutKeys.hinted($t('workspaceUi.composer.gifs'), 'gifs')"
          side="top"
        >
          <UiButton
            :ref="(el) => (gifTrigger = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
            variant="ghost"
            size="icon-sm"
            type="button"
            class="composer-gif-button composer-expression"
            :aria-label="$t('workspaceUi.composer.chooseGif')"
            :aria-keyshortcuts="shortcutKeys.aria('gifs')"
            :aria-expanded="pickerTab === 'gifs'"
            :class="{ 'is-open': pickerTab === 'gifs' }"
            :disabled="!enabled"
            @click="togglePicker('gifs')"
          >
            <span
              class="gif-button-label text-[11px] font-[750] tracking-[0.02em] py-[1px] px-1 [border:1.5px_solid_currentColor] rounded-[4px] leading-[1.2]"
              >GIF</span
            >
          </UiButton>
        </AppTooltip>
        <AppTooltip
          :text="$t('workspaceUi.composer.emoji')"
          side="top"
        >
          <UiButton
            :ref="(el) => (emojiTrigger = (el as { $el: HTMLButtonElement } | null)?.$el ?? null)"
            variant="ghost"
            size="icon-sm"
            type="button"
            class="composer-expression"
            :aria-label="$t('workspaceUi.composer.chooseEmoji')"
            :aria-expanded="pickerTab === 'emoji'"
            :class="{ 'is-open': pickerTab === 'emoji' }"
            :disabled="!enabled"
            @click="togglePicker('emoji')"
          >
            <AppIcon
              name="smile"
              :size="20"
            />
          </UiButton>
        </AppTooltip>
        <button
          v-if="draft.trim() || uploads.pending.value.length"
          class="send-button inline-flex items-center justify-center w-8 h-7.5 text-primary rounded-sm hover:not-disabled:bg-accent [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
          type="submit"
          :disabled="!enabled"
          :aria-label="$t('workspaceUi.composer.send')"
          :title="$t('workspaceUi.composer.send')"
        >
          <AppIcon
            name="plane"
            :size="18"
          />
        </button>
      </div>
    </form>
    <p
      v-if="gifQuery"
      class="typing-line absolute bottom-full left-4.5 right-4 flex items-center gap-1.5 h-5 m-0 py-0 px-1.5 overflow-hidden [background:linear-gradient(to_top,_var(--main)_65%,_transparent)] pointer-events-none text-muted-foreground text-[0.78rem] whitespace-nowrap text-ellipsis gif-lookup"
    >
      <span
        class="typing-dots inline-flex gap-[3px]"
        aria-hidden="true"
        ><i /><i /><i /></span
      >{{ $t('workspaceUi.composer.findingGif', { query: gifQuery }) }}
    </p>
    <p
      v-else-if="typingText"
      class="typing-line absolute bottom-full left-4.5 right-4 flex items-center gap-1.5 h-5 m-0 py-0 px-1.5 overflow-hidden [background:linear-gradient(to_top,_var(--main)_65%,_transparent)] pointer-events-none text-muted-foreground text-[0.78rem] whitespace-nowrap text-ellipsis"
    >
      <span
        class="typing-dots inline-flex gap-[3px]"
        aria-hidden="true"
        ><i /><i /><i /></span
      >{{ typingText }}
    </p>
    <ExpressionPicker
      v-if="pickerTab && composerElement"
      :anchor="composerElement"
      :tab="pickerTab"
      @update:tab="pickerTab = $event"
      @close="closePicker"
      @emoji="insertEmoji"
      @gif="sendGif"
    />
    <span
      :id="inputHelp"
      class="sr-only"
      >{{ $t('workspaceUi.composer.inputHelp')
      }}<template v-if="!rawMode && gifHint">
        {{ $t('workspaceUi.composer.gifShortcut', { shortcut: gifHint }) }}</template
      ></span
    >
  </div>
</template>
