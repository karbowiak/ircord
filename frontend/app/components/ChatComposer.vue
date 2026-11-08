<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import type { EmojiItem, GifItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import { stripFormatting } from '../../shared/utils/irc'
import { clipboardListText, normalizeBullets } from '../../shared/utils/lists'
import { ariaShortcut, isMac, shortcutLabel, shortcutOf } from '../../shared/utils/shortcuts'
import { overlayOpen } from '../../shared/utils/focus'
const props = defineProps<{ replyTo?: Message }>()
const emit = defineEmits<{ cancelReply: [] }>()
const chat = useChatStore()
const library = useLibraryStore()
const media = useMediaStore()
const uploads = useUploads()
const { run, gifLookup } = useCommands()
const { open: openMenu } = useContextMenu()
const nameStyle = useNameStyle()
const input = ref<HTMLTextAreaElement | null>(null)
useTypeToCompose(input)
const fileInput = ref<HTMLInputElement | null>(null)
const composerElement = ref<HTMLFormElement | null>(null)
const gifTrigger = ref<HTMLButtonElement | null>(null)
const emojiTrigger = ref<HTMLButtonElement | null>(null)
const pickerTab = ref<'gifs' | 'emoji' | null>(null)
/**
 * Where focus goes when the picker is dismissed: its trigger button when it was clicked open,
 * the message box (with its caret) when it was opened from the keyboard.
 */
let pickerReturn: 'trigger' | { start: number; end: number } = 'trigger'
const mac = isMac(navigator.platform)
const gifHint = shortcutLabel('gifs', mac)
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
    ? (chat.network?.name ?? 'server')
    : chat.active?.kind === 'channel'
      ? `#${chat.active.name}`
      : `@${chat.active ? (chat.directPartner(chat.active)?.nick ?? chat.active.name) : ''}`,
)
const placeholder = computed(() =>
  !enabled.value
    ? chat.status !== 'connected' || !chat.network
      ? 'Waiting for connection…'
      : chat.network.status === 'connecting'
        ? `Connecting to ${chat.network.name}…`
        : chat.network.status === 'error'
          ? `${chat.network.name}: ${chat.network.statusDetail ?? 'connection failed'}`
          : (chat.sendReason ?? `${chat.network.name} is disconnected`)
    : rawMode.value
      ? `Send a command to ${target.value}, e.g. WHOIS jules`
      : `Message ${target.value}`,
)

/** The query of a /gif lookup running for the open conversation; shown until it finishes. */
const gifQuery = computed(() => gifLookup(chat.activeId))
const typingText = computed(() => {
  const nicks = (chat.typing[chat.activeId] ?? []).map((id) => chat.member(id)?.nick ?? id)
  if (!nicks.length) return ''
  if (nicks.length === 1) return `${nicks[0]} is typing…`
  if (nicks.length <= 3) return `${nicks.slice(0, -1).join(', ')} and ${nicks.at(-1)} are typing…`
  return 'Several people are typing…'
})
const auto = useComposerAutocomplete({ draft, input, rawMode, setDraft })
const { token, index: suggestionIndex, count: suggestionCount, complete } = auto
const { emoji: emojiSuggestions, members: memberSuggestions } = auto.suggestions
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

function onInput() {
  auto.onInput()
  resize()
  if (!rawMode.value && draft.value.trim() && !draft.value.startsWith('/'))
    chat.sendTyping('active')
  else chat.sendTyping('done')
}
function resize() {
  if (input.value) {
    input.value.style.height = 'auto'
    input.value.style.height = `${Math.min(input.value.scrollHeight, 200)}px`
  }
}

// ---- Sending ----------------------------------------------------------------------------------

async function send() {
  const text = draft.value
  const files = uploads.pending.value.length
  if (!text.trim() && !files) return
  if (text.length > 4000) return chat.notify('Messages can be up to 4,000 characters.')
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
  if (message.trim() && !chat.send(message, props.replyTo)) return
  clear()
  if (files) void uploads.sendAll()
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

function keydown(event: KeyboardEvent) {
  if (auto.keydown(event)) return
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) {
    event.preventDefault()
    void send()
  }
  const format = shortcutOf(event, ['bold', 'italic', 'underline'])
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
    chat.notify('This message is at its character limit.')
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
    input.value?.setSelectionRange(back.start, back.end)
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
  const caret = { start: input.value.selectionStart, end: input.value.selectionEnd }
  pickerReturn =
    document.activeElement === input.value
      ? caret
      : { start: draft.value.length, end: draft.value.length }
  pickerTab.value = 'gifs'
  return true
}
const actions = useWorkspaceActions()
actions.gifs = gifShortcut
onBeforeUnmount(() => {
  if (actions.gifs === gifShortcut) actions.gifs = () => false
})

// ---- Attachments ------------------------------------------------------------------------------

function plusMenu(event: MouseEvent) {
  openMenu(event.currentTarget as HTMLElement, [
    {
      label: 'Upload a file',
      icon: 'upload',
      disabled: !uploads.capability.value,
      hint: uploads.capability.value
        ? formatBytes(uploads.capability.value.maxBytes)
        : 'unavailable',
      action: () => fileInput.value?.click(),
    },
    { label: 'Send a GIF', icon: 'gif', hint: gifHint, action: () => togglePicker('gifs') },
    {
      label: 'Use a command',
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
  const html = data?.getData('text/html') ?? ''
  if (/<li[\s>]/i.test(html)) {
    const text = clipboardListText(new DOMParser().parseFromString(html, 'text/html').body)
    if (text) return text
  }
  const plain = data?.getData('text/plain') ?? ''
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
  /** Returns false when the box can't take focus (disconnected), so the caller can focus elsewhere. */
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
  <div class="composer-area">
    <div
      v-if="replyTo"
      class="composer-reply"
    >
      <AppIcon
        name="reply"
        :size="15"
      />
      <span
        >Replying to
        <strong :style="nameStyle(chat.member(replyTo.senderId))">{{
          chat.member(replyTo.senderId)?.displayName
        }}</strong></span
      >
      <span class="reply-excerpt">{{ stripFormatting(replyTo.text) }}</span>
      <button
        class="icon-button"
        aria-label="Cancel reply"
        @click="emit('cancelReply')"
      >
        <AppIcon
          name="close"
          :size="16"
        />
      </button>
    </div>
    <ComposerSuggestions
      v-if="token && suggestionCount"
      :kind="token.kind"
      :emoji="emojiSuggestions"
      :members="memberSuggestions"
      :commands="commandSuggestions"
      :channels="channelSuggestions"
      :active="suggestionIndex"
      :query="token.query"
      :list-id="suggestionsId"
      @emoji="complete.emoji"
      @member="complete.member"
      @command="complete.command"
      @channel="complete.channel"
      @hover="suggestionIndex = $event"
    />
    <ComposerUploads />
    <form
      ref="composerElement"
      class="composer"
      :class="{ 'composer-disabled': !enabled, 'is-raw': rawMode }"
      :aria-busy="!!gifQuery || undefined"
      @submit.prevent="send"
    >
      <button
        v-if="!rawMode"
        type="button"
        class="icon-button composer-plus"
        aria-label="Attach or add"
        title="Upload a file, GIF or command"
        :disabled="!enabled"
        @click="plusMenu"
      >
        <AppIcon
          name="plus"
          :size="20"
        />
      </button>
      <span
        v-else
        class="raw-prompt"
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
      <!-- Drafts are saved per conversation, so an update reload may happen while it has text. -->
      <textarea
        ref="input"
        v-model="draft"
        data-saved-draft
        :aria-label="rawMode ? `Command to ${target}` : `Message ${target}`"
        :aria-describedby="inputHelp"
        :aria-keyshortcuts="rawMode ? undefined : ariaShortcut('gifs', mac)"
        :placeholder="placeholder"
        rows="1"
        :maxlength="4000"
        :disabled="!enabled"
        role="combobox"
        aria-autocomplete="list"
        :aria-expanded="suggestionCount > 0"
        :aria-controls="suggestionCount ? suggestionsId : undefined"
        :aria-activedescendant="suggestionCount ? `${suggestionsId}-${suggestionIndex}` : undefined"
        @input="onInput"
        @keydown="keydown"
        @keyup="auto.trackCaret"
        @click="auto.trackCaret"
        @focus="media.needAll()"
        @blur="auto.dismiss"
        @paste="paste"
      />
      <div
        v-if="!rawMode"
        class="composer-actions"
      >
        <span
          v-if="draft.length > 3600"
          class="character-count"
          >{{ 4000 - draft.length }}</span
        >
        <button
          ref="gifTrigger"
          type="button"
          class="icon-button composer-gif-button"
          aria-label="Choose a GIF"
          :title="`GIFs (${gifHint})`"
          :aria-keyshortcuts="ariaShortcut('gifs', mac)"
          :aria-expanded="pickerTab === 'gifs'"
          :class="{ 'is-open': pickerTab === 'gifs' }"
          :disabled="!enabled"
          @click="togglePicker('gifs')"
        >
          <span class="gif-button-label">GIF</span>
        </button>
        <button
          ref="emojiTrigger"
          type="button"
          class="icon-button"
          aria-label="Choose an emoji"
          title="Emoji"
          :aria-expanded="pickerTab === 'emoji'"
          :class="{ 'is-open': pickerTab === 'emoji' }"
          :disabled="!enabled"
          @click="togglePicker('emoji')"
        >
          <AppIcon
            name="smile"
            :size="20"
          />
        </button>
        <button
          v-if="draft.trim() || uploads.pending.value.length"
          class="send-button"
          type="submit"
          :disabled="!enabled"
          aria-label="Send message"
          title="Send message"
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
      class="typing-line gif-lookup"
      role="status"
    >
      <span
        class="typing-dots"
        aria-hidden="true"
        ><i /><i /><i /></span
      >Finding a GIF for “{{ gifQuery }}”…
    </p>
    <p
      v-else-if="typingText"
      class="typing-line"
    >
      <span
        class="typing-dots"
        aria-hidden="true"
        ><i /><i /><i /></span
      >{{ typingText }}
    </p>
    <LazyExpressionPicker
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
      >Enter to send, Shift and Enter for a new line. Type / for commands, a colon for emoji, @ to
      mention someone, or press Tab to complete a nickname.<template v-if="!rawMode">
        Press {{ gifHint }} for GIFs.</template
      ></span
    >
  </div>
</template>
