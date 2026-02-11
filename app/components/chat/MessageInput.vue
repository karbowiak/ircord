<template>
  <div ref="wrapperEl" class="message-input-wrap">
    <GifPicker
      :is-open="isGifPickerOpen"
      @close="isGifPickerOpen = false"
      @select="onGifSelect"
    />

    <EmojiPicker
      :is-open="isEmojiPickerOpen"
      @close="isEmojiPickerOpen = false"
      @select="onEmojiSelect"
    />

    <div v-if="appStore.replyTargetMessage" class="reply-banner">
      <span class="reply-label">Replying to {{ appStore.replyTargetMessage.author.username }}</span>
      <span class="reply-snippet">{{ appStore.replyTargetMessage.content }}</span>
      <button type="button" class="reply-cancel" @click="appStore.setReplyTarget(null)">×</button>
    </div>

    <div v-if="isChannelUnavailable" class="channel-state-banner" :class="channelStateClass">
      <span class="channel-state-text">{{ channelStateText }}</span>
      <button
        v-if="canRejoin"
        type="button"
        class="channel-state-action"
        @click="rejoinChannel"
      >
        Rejoin
      </button>
    </div>

    <div class="message-input">
      <div v-if="slashAutocompleteOpen && slashSuggestions.length" class="slash-autocomplete">
        <button
          v-for="(item, idx) in slashSuggestions"
          :key="item.command"
          type="button"
          class="slash-option"
          :class="{ active: idx === activeSlashSuggestionIndex }"
          @mousedown.prevent
          @click="applySlashSuggestion(item.command)"
        >
          <span class="slash-option-command">/{{ item.command }}</span>
          <span class="slash-option-help">{{ item.help }}</span>
        </button>
      </div>

      <div v-if="mentionAutocompleteOpen && mentionSuggestions.length" class="mention-autocomplete">
        <button
          v-for="(nick, idx) in mentionSuggestions"
          :key="nick"
          type="button"
          class="mention-option"
          :class="{ active: idx === activeMentionSuggestionIndex }"
          @mousedown.prevent
          @click="applyMentionSuggestion(nick)"
        >
          <span class="mention-option-tag">@{{ nick }}</span>
        </button>
      </div>

      <div v-if="emojiAutocompleteOpen && emojiSuggestions.length" class="emoji-autocomplete">
        <button
          v-for="(item, idx) in emojiSuggestions"
          :key="item.shortcode"
          type="button"
          class="emoji-option"
          :class="{ active: idx === activeEmojiSuggestionIndex }"
          @mousedown.prevent
          @click="applyEmojiSuggestion(item)"
        >
          <img
            v-if="item.imageUrl"
            :src="item.imageUrl"
            :alt="item.shortcode"
            class="emoji-option-image"
            loading="lazy"
          >
          <span v-else class="emoji-option-char">{{ item.char }}</span>
          <span class="emoji-option-shortcode">:{{ item.shortcode }}:</span>
        </button>
      </div>

      <span class="nick-prefix">{{ displayNick }}&gt;</span>
      <input
        ref="inputEl"
        v-model="messageText"
        type="text"
        :placeholder="placeholder"
        class="input"
        :disabled="isChannelUnavailable"
        @keydown="onInputKeydown"
        @input="updateInlineAutocomplete"
        @click="updateInlineAutocomplete"
      >

      <div class="right-controls">
        <button
          type="button"
          class="icon-button"
          title="Open emoji picker"
          :disabled="isChannelUnavailable"
          @click="toggleEmojiPicker"
        >
          :)
        </button>

        <button
          type="button"
          class="icon-button"
          title="Open GIF picker"
          :disabled="isChannelUnavailable"
          @click="toggleGifPicker"
        >
          GIF
        </button>

        <button
          type="button"
          class="icon-button send"
          title="Send"
          :disabled="isChannelUnavailable"
          @click="sendMessage"
        >
          Send
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import GifPicker from '~/components/chat/GifPicker.vue'
import EmojiPicker from '~/components/chat/EmojiPicker.vue'
import { emojiDefinitions, findEmojiByShortcode, normalizeEmoticonsInText } from '~/composables/useEmojiData'

const appStore = useAppStore()
const messageText = ref('')
const inputEl = ref<HTMLInputElement>()
const wrapperEl = ref<HTMLDivElement>()
const isGifPickerOpen = ref(false)
const isEmojiPickerOpen = ref(false)
const activeEmojiSuggestionIndex = ref(0)
const emojiAutocompleteOpen = ref(false)
const emojiAutocompleteQuery = ref('')
const emojiReplaceRange = ref<{ start: number, end: number } | null>(null)
const activeMentionSuggestionIndex = ref(0)
const mentionAutocompleteOpen = ref(false)
const mentionAutocompleteQuery = ref('')
const mentionReplaceRange = ref<{ start: number, end: number } | null>(null)
const activeSlashSuggestionIndex = ref(0)
const slashAutocompleteOpen = ref(false)
const slashAutocompleteQuery = ref('')
const slashReplaceRange = ref<{ start: number, end: number } | null>(null)

interface EmojiSuggestion {
  shortcode: string
  char?: string
  imageUrl?: string
  keywords: string[]
}

type SlashCommandSuggestion = {
  command: string
  help: string
}

const slashCommandSuggestions = [
  { command: 'join', help: 'Join a channel' },
  { command: 'part', help: 'Leave current/target channel' },
  { command: 'msg', help: 'Send private message' },
  { command: 'slap', help: 'Slap user with trout' },
  { command: 'kick', help: 'Kick user from channel' },
  { command: 'ban', help: 'Ban user from channel' },
  { command: 'shrug', help: 'Send shrug emote' },
  { command: 'tableflip', help: 'Send table flip emote' },
] as const satisfies readonly SlashCommandSuggestion[]

const emojiSuggestions = computed<EmojiSuggestion[]>(() => {
  const query = emojiAutocompleteQuery.value.trim().toLowerCase()
  if (!query) return []

  return emojiDefinitions
    .filter((item) => {
      return item.shortcode.includes(query)
        || item.keywords.some((keyword) => keyword.includes(query))
    })
    .slice(0, 8)
})

const displayNick = computed(() => appStore.activeServerStatus?.nick || appStore.currentUser.username)

const mentionCandidates = computed(() => {
  const channel = appStore.activeChannel
  if (!channel) return [] as string[]

  if ('members' in channel) {
    return [...new Set(channel.members.map(entry => entry.user.username))]
  }

  return [...new Set([channel.recipient.username, displayNick.value])]
})

const mentionSuggestions = computed(() => {
  const query = mentionAutocompleteQuery.value.trim().toLowerCase()
  if (!query) return [] as string[]

  return mentionCandidates.value
    .filter(nick => nick.toLowerCase().startsWith(query))
    .slice(0, 8)
})

const slashSuggestions = computed<SlashCommandSuggestion[]>(() => {
  const query = slashAutocompleteQuery.value.trim().toLowerCase()
  return slashCommandSuggestions
    .filter(item => !query || item.command.startsWith(query))
    .slice(0, 8)
})

watch(emojiSuggestions, (items) => {
  if (!items.length) {
    emojiAutocompleteOpen.value = false
    activeEmojiSuggestionIndex.value = 0
    return
  }

  if (activeEmojiSuggestionIndex.value >= items.length) {
    activeEmojiSuggestionIndex.value = 0
  }
})

watch(mentionSuggestions, (items) => {
  if (!items.length) {
    mentionAutocompleteOpen.value = false
    activeMentionSuggestionIndex.value = 0
    return
  }

  if (activeMentionSuggestionIndex.value >= items.length) {
    activeMentionSuggestionIndex.value = 0
  }
})

watch(slashSuggestions, (items) => {
  if (!items.length) {
    slashAutocompleteOpen.value = false
    activeSlashSuggestionIndex.value = 0
    return
  }

  if (activeSlashSuggestionIndex.value >= items.length) {
    activeSlashSuggestionIndex.value = 0
  }
})

const isTouchDevice = computed(() => {
  if (!import.meta.client) return false
  return navigator.maxTouchPoints > 0
})

const isMacPlatform = computed(() => {
  if (!import.meta.client) return false

  const ua = navigator.userAgent.toLowerCase()
  const platform = navigator.platform.toLowerCase()
  return platform.includes('mac') || ua.includes('mac') || ua.includes('iphone') || ua.includes('ipad')
})

const placeholder = computed(() => {
  if (isChannelUnavailable.value) {
    return canRejoin.value ? 'You were removed from this channel' : 'You cannot send messages in this channel'
  }

  const channel = appStore.activeChannel
  if (!channel) return 'Type a message...'
  if ('name' in channel) {
    return `Message #${channel.name}... (or /command)`
  }
  return `Message ${channel.recipient.username}...`
})

const channelParticipation = computed(() => appStore.activeChannelParticipation)

const isChannelUnavailable = computed(() => {
  const channel = appStore.activeChannel
  if (!channel || !('members' in channel)) return false
  return channelParticipation.value?.joined === false
})

const canRejoin = computed(() => {
  if (!isChannelUnavailable.value) return false
  return Boolean(channelParticipation.value?.canRejoin)
})

const channelStateText = computed(() => {
  if (!isChannelUnavailable.value) return ''
  const reason = channelParticipation.value?.reason
  const detail = channelParticipation.value?.detail

  if (reason === 'ban') {
    return detail ? `You are banned from this channel (${detail})` : 'You are banned from this channel'
  }

  if (reason === 'kick') {
    return detail ? `You were kicked (${detail})` : 'You were kicked from this channel'
  }

  if (reason === 'part') {
    return detail || 'You left this channel'
  }

  return 'You are no longer in this channel'
})

const channelStateClass = computed(() => channelParticipation.value?.reason === 'ban' ? 'banned' : 'kicked')

async function sendMessage() {
  if (isChannelUnavailable.value) return
  const normalizedText = normalizeEmoticonsInText(messageText.value)

  if (normalizedText.trim().startsWith('/')) {
    const handled = await executeSlashCommand(normalizedText.trim())
    if (!handled) return

    messageText.value = ''
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    closeMentionAutocomplete()
    closeSlashAutocomplete()
    nextTick(() => inputEl.value?.focus())
    return
  }

  if (appStore.sendMockMessage(normalizedText)) {
    messageText.value = ''
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    closeMentionAutocomplete()
    closeSlashAutocomplete()
    nextTick(() => inputEl.value?.focus())
  }
}

async function rejoinChannel() {
  await appStore.rejoinActiveChannel()
}

function toggleGifPicker() {
  closeEmojiAutocomplete()
  closeMentionAutocomplete()
  closeSlashAutocomplete()
  isEmojiPickerOpen.value = false
  isGifPickerOpen.value = !isGifPickerOpen.value
}

function toggleEmojiPicker() {
  closeEmojiAutocomplete()
  closeMentionAutocomplete()
  closeSlashAutocomplete()
  isGifPickerOpen.value = false
  isEmojiPickerOpen.value = !isEmojiPickerOpen.value
}

function onGifSelect(url: string) {
  if (appStore.sendMockMessage(url)) {
    isGifPickerOpen.value = false
    closeEmojiAutocomplete()
    closeMentionAutocomplete()
    closeSlashAutocomplete()
    nextTick(() => inputEl.value?.focus())
  }
}

function onEmojiSelect(shortcode: string) {
  const resolved = findEmojiByShortcode(shortcode)
  const insertion = resolved?.char ?? shortcode

  const withSpacing = messageText.value.endsWith(' ') || !messageText.value
    ? insertion
    : ` ${insertion}`

  messageText.value = `${messageText.value}${withSpacing} `
  isEmojiPickerOpen.value = false
  closeEmojiAutocomplete()
  closeMentionAutocomplete()
  closeSlashAutocomplete()
  nextTick(() => inputEl.value?.focus())
}

function closeEmojiAutocomplete() {
  emojiAutocompleteOpen.value = false
  emojiAutocompleteQuery.value = ''
  activeEmojiSuggestionIndex.value = 0
  emojiReplaceRange.value = null
}

function closeMentionAutocomplete() {
  mentionAutocompleteOpen.value = false
  mentionAutocompleteQuery.value = ''
  activeMentionSuggestionIndex.value = 0
  mentionReplaceRange.value = null
}

function closeSlashAutocomplete() {
  slashAutocompleteOpen.value = false
  slashAutocompleteQuery.value = ''
  activeSlashSuggestionIndex.value = 0
  slashReplaceRange.value = null
}

function updateMentionAutocomplete() {
  if (!inputEl.value) {
    closeMentionAutocomplete()
    return
  }

  const caretPosition = inputEl.value.selectionStart ?? messageText.value.length
  const beforeCaret = messageText.value.slice(0, caretPosition)
  const trigger = beforeCaret.match(/(^|\s)@([A-Za-z0-9_\-\[\]\\`^{}|]{1,32})$/)

  if (!trigger) {
    closeMentionAutocomplete()
    return
  }

  const query = trigger[2]
  const tokenStart = caretPosition - query.length - 1

  mentionAutocompleteQuery.value = query
  mentionReplaceRange.value = { start: tokenStart, end: caretPosition }
  activeMentionSuggestionIndex.value = 0
  mentionAutocompleteOpen.value = true
}

function applyMentionSuggestion(nick: string) {
  if (!mentionReplaceRange.value) return

  const replacement = `@${nick}`
  const { start, end } = mentionReplaceRange.value
  const suffix = messageText.value.slice(end)
  const appendSpace = suffix.startsWith(' ') || suffix.length === 0 ? '' : ' '
  const nextText = `${messageText.value.slice(0, start)}${replacement}${appendSpace}${suffix}`
  const nextCaret = start + replacement.length + appendSpace.length

  messageText.value = nextText
  closeMentionAutocomplete()

  nextTick(() => {
    if (!inputEl.value) return
    inputEl.value.focus()
    inputEl.value.setSelectionRange(nextCaret, nextCaret)
  })
}

function updateSlashAutocomplete() {
  if (!inputEl.value) {
    closeSlashAutocomplete()
    return
  }

  const caretPosition = inputEl.value.selectionStart ?? messageText.value.length
  const beforeCaret = messageText.value.slice(0, caretPosition)
  const trigger = beforeCaret.match(/^\/([a-z]*)$/i)

  if (!trigger) {
    closeSlashAutocomplete()
    return
  }

  slashAutocompleteQuery.value = (trigger[1] || '').toLowerCase()
  slashReplaceRange.value = { start: 0, end: caretPosition }
  activeSlashSuggestionIndex.value = 0
  slashAutocompleteOpen.value = true
}

function applySlashSuggestion(command: string) {
  if (!slashReplaceRange.value) return

  const replacement = `/${command} `
  const { start, end } = slashReplaceRange.value
  const nextText = `${messageText.value.slice(0, start)}${replacement}${messageText.value.slice(end)}`
  const nextCaret = start + replacement.length

  messageText.value = nextText
  closeSlashAutocomplete()

  nextTick(() => {
    if (!inputEl.value) return
    inputEl.value.focus()
    inputEl.value.setSelectionRange(nextCaret, nextCaret)
  })
}

function updateInlineAutocomplete() {
  updateSlashAutocomplete()
  updateMentionAutocomplete()
  updateEmojiAutocomplete()
}

function updateEmojiAutocomplete() {
  if (!inputEl.value) {
    closeEmojiAutocomplete()
    return
  }

  const caretPosition = inputEl.value.selectionStart ?? messageText.value.length
  const beforeCaret = messageText.value.slice(0, caretPosition)
  const trigger = beforeCaret.match(/(^|\s):([a-z0-9_+-]{1,32})$/i)

  if (!trigger) {
    closeEmojiAutocomplete()
    return
  }

  const query = trigger[2].toLowerCase()
  const tokenStart = caretPosition - query.length - 1

  emojiAutocompleteQuery.value = query
  emojiReplaceRange.value = { start: tokenStart, end: caretPosition }
  activeEmojiSuggestionIndex.value = 0
  emojiAutocompleteOpen.value = true
}

function applyEmojiSuggestion(item: EmojiSuggestion) {
  if (!emojiReplaceRange.value) return

  const replacement = item.char ?? `:${item.shortcode}:`
  const { start, end } = emojiReplaceRange.value
  const nextText = `${messageText.value.slice(0, start)}${replacement}${messageText.value.slice(end)}`
  const nextCaret = start + replacement.length

  messageText.value = nextText
  closeEmojiAutocomplete()

  nextTick(() => {
    if (!inputEl.value) return
    inputEl.value.focus()
    inputEl.value.setSelectionRange(nextCaret, nextCaret)
  })
}

function onInputKeydown(event: KeyboardEvent) {
  if (slashAutocompleteOpen.value && slashSuggestions.value.length > 0) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      activeSlashSuggestionIndex.value = (activeSlashSuggestionIndex.value + 1) % slashSuggestions.value.length
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      activeSlashSuggestionIndex.value = (activeSlashSuggestionIndex.value - 1 + slashSuggestions.value.length) % slashSuggestions.value.length
      return
    }

    if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault()
      const selected = slashSuggestions.value[activeSlashSuggestionIndex.value]
      if (selected) {
        applySlashSuggestion(selected.command)
      }
      return
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      closeSlashAutocomplete()
      return
    }
  }

  if (mentionAutocompleteOpen.value && mentionSuggestions.value.length > 0) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      activeMentionSuggestionIndex.value = (activeMentionSuggestionIndex.value + 1) % mentionSuggestions.value.length
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      activeMentionSuggestionIndex.value = (activeMentionSuggestionIndex.value - 1 + mentionSuggestions.value.length) % mentionSuggestions.value.length
      return
    }

    if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault()
      const selected = mentionSuggestions.value[activeMentionSuggestionIndex.value]
      if (selected) {
        applyMentionSuggestion(selected)
      }
      return
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      closeMentionAutocomplete()
      return
    }
  }

  if (emojiAutocompleteOpen.value && emojiSuggestions.value.length > 0) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      activeEmojiSuggestionIndex.value = (activeEmojiSuggestionIndex.value + 1) % emojiSuggestions.value.length
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      activeEmojiSuggestionIndex.value = (activeEmojiSuggestionIndex.value - 1 + emojiSuggestions.value.length) % emojiSuggestions.value.length
      return
    }

    if (event.key === 'Enter' || event.key === 'Tab') {
      event.preventDefault()
      const selected = emojiSuggestions.value[activeEmojiSuggestionIndex.value]
      if (selected) {
        applyEmojiSuggestion(selected)
      }
      return
    }

    if (event.key === 'Escape') {
      event.preventDefault()
      closeEmojiAutocomplete()
      return
    }
  }

  if (event.key === 'Enter') {
    event.preventDefault()
    void sendMessage()
  }
}

async function executeSlashCommand(raw: string): Promise<boolean> {
  const parts = raw.slice(1).trim().split(/\s+/)
  const command = (parts[0] || '').toLowerCase()
  const args = parts.slice(1)

  if (!command) return false

  const activeChannel = appStore.activeChannel
  const activeChannelId = appStore.activeChannelId

  if (command === 'shrug') {
    return appStore.sendMockMessage('¯\\_(ツ)_/¯')
  }

  if (command === 'tableflip') {
    return appStore.sendMockMessage('(╯°□°)╯︵ ┻━┻')
  }

  if (command === 'join') {
    const channelName = args[0]
    if (!channelName) return false
    return appStore.joinIrcChannel(channelName)
  }

  if (command === 'part') {
    const channelArg = args[0]
    if (channelArg) {
      const target = channelArg.startsWith('#') ? channelArg.slice(1) : channelArg
      const channelId = appStore.channelsForActiveServer.find(entry => entry.name.toLowerCase() === target.toLowerCase())?.id
      if (!channelId) return false
      return appStore.leaveIrcChannel(channelId)
    }

    if (!activeChannelId || !activeChannel || !('members' in activeChannel)) return false
    return appStore.leaveIrcChannel(activeChannelId)
  }

  if (command === 'msg') {
    const target = args[0]
    const text = args.slice(1).join(' ').trim()
    if (!target || !text) return false
    return appStore.sendIrcPrivateMessage(target, text)
  }

  if (!activeChannelId || !activeChannel || !('members' in activeChannel)) return false

  if (command === 'slap') {
    const target = args[0]
    if (!target) return false
    return appStore.slapChannelMember(activeChannelId, target)
  }

  if (command === 'kick') {
    const target = args[0]?.replace(/^@/, '')
    if (!target) return false
    return appStore.kickChannelMember(activeChannelId, target)
  }

  if (command === 'ban') {
    const target = args[0]?.replace(/^@/, '')
    if (!target) return false
    return appStore.banChannelMember(activeChannelId, target)
  }

  return false
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (isTouchDevice.value) return

  const lower = event.key.toLowerCase()
  const primaryCombo = isMacPlatform.value
    ? event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey
    : event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey

  const shortcutScopeActive = import.meta.client && document.hasFocus()

  if (primaryCombo && lower === 'g' && shortcutScopeActive) {
    event.preventDefault()
    event.stopPropagation()
    toggleGifPicker()
    return
  }

  if (lower === 'escape' && (isGifPickerOpen.value || isEmojiPickerOpen.value || emojiAutocompleteOpen.value || mentionAutocompleteOpen.value || slashAutocompleteOpen.value)) {
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    closeMentionAutocomplete()
    closeSlashAutocomplete()
    nextTick(() => inputEl.value?.focus())
  }
}

function onGlobalPointerDown(event: MouseEvent) {
  if (!wrapperEl.value) return
  if (wrapperEl.value.contains(event.target as Node)) return

  if (isGifPickerOpen.value || isEmojiPickerOpen.value || emojiAutocompleteOpen.value || mentionAutocompleteOpen.value || slashAutocompleteOpen.value) {
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    closeMentionAutocomplete()
    closeSlashAutocomplete()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown, { capture: true })
  window.addEventListener('mousedown', onGlobalPointerDown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKeydown, { capture: true })
  window.removeEventListener('mousedown', onGlobalPointerDown, { capture: true })
})
</script>

<style scoped>
.message-input-wrap {
  position: relative;
}

.reply-banner {
  margin: 0 16px;
  margin-bottom: 6px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0, 0, 0, 0.2);
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.channel-state-banner {
  margin: 0 16px;
  margin-bottom: 6px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 7px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.channel-state-banner.kicked {
  background: rgba(166, 124, 0, 0.18);
  border-color: rgba(255, 201, 107, 0.34);
}

.channel-state-banner.banned {
  background: rgba(145, 39, 39, 0.22);
  border-color: rgba(255, 120, 120, 0.4);
}

.channel-state-text {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
}

.channel-state-action {
  border: 0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.14);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 6px 10px;
  cursor: pointer;
}

.channel-state-action:hover {
  background: rgba(255, 255, 255, 0.2);
}

.reply-label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #c9d5ff;
  white-space: nowrap;
}

.reply-snippet {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.reply-cancel {
  border: 0;
  border-radius: 999px;
  width: 18px;
  height: 18px;
  line-height: 1;
  font-size: 12px;
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-body);
  cursor: pointer;
}

.message-input {
  position: relative;
  height: 44px;
  background-color: var(--bg-input);
  display: flex;
  align-items: center;
  padding: 0 12px;
  margin: 0 16px 16px 16px;
  border-radius: 8px;
  gap: 8px;
}

.emoji-autocomplete {
  position: absolute;
  left: 52px;
  right: 154px;
  bottom: calc(100% + 10px);
  max-height: 220px;
  overflow-y: auto;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(180deg, rgba(40, 43, 50, 0.98), rgba(36, 39, 46, 0.98));
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.42);
  z-index: 45;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.slash-autocomplete {
  position: absolute;
  left: 52px;
  right: 154px;
  bottom: calc(100% + 10px);
  max-height: 240px;
  overflow-y: auto;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(180deg, rgba(40, 43, 50, 0.98), rgba(36, 39, 46, 0.98));
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.42);
  z-index: 47;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.slash-option {
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-body);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 8px;
  cursor: pointer;
  text-align: left;
}

.slash-option:hover,
.slash-option.active {
  background-color: rgba(88, 101, 242, 0.2);
}

.slash-option-command {
  font-family: var(--font-mono);
  font-size: 12px;
  color: #b5c7ff;
}

.slash-option-help {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
}

.mention-autocomplete {
  position: absolute;
  left: 52px;
  right: 154px;
  bottom: calc(100% + 10px);
  max-height: 220px;
  overflow-y: auto;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: linear-gradient(180deg, rgba(40, 43, 50, 0.98), rgba(36, 39, 46, 0.98));
  box-shadow: 0 14px 30px rgba(0, 0, 0, 0.42);
  z-index: 46;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.mention-option {
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-body);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  cursor: pointer;
  text-align: left;
}

.mention-option:hover,
.mention-option.active {
  background-color: rgba(88, 101, 242, 0.2);
}

.mention-option-tag {
  font-family: var(--font-mono);
  font-size: 12px;
  color: #9bb4ff;
}

.emoji-option {
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-body);
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  cursor: pointer;
  text-align: left;
}

.emoji-option:hover,
.emoji-option.active {
  background-color: rgba(88, 101, 242, 0.2);
}

.emoji-option-char {
  width: 22px;
  text-align: center;
  font-size: 18px;
}

.emoji-option-image {
  width: 22px;
  height: 22px;
  object-fit: contain;
}

.emoji-option-shortcode {
  font-family: var(--font-mono);
  font-size: 12px;
}

.icon-button {
  border: 0;
  border-radius: 6px;
  background-color: rgba(255, 255, 255, 0.06);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  padding: 5px 8px;
  cursor: pointer;
}

.icon-button:hover {
  background-color: rgba(255, 255, 255, 0.12);
}

.icon-button:disabled {
  opacity: 0.45;
  cursor: default;
}

.icon-button.send {
  color: var(--accent-green);
}

.icon-button:focus-visible {
  outline: 2px solid rgba(88, 101, 242, 0.7);
  outline-offset: 1px;
}

.right-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}

.nick-prefix {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  color: var(--accent-green);
  white-space: nowrap;
  user-select: none;
}

.input {
  flex: 1;
  height: 100%;
  background: transparent;
  border: none;
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 14px;
  outline: none;
}

.input::placeholder {
  color: var(--text-faint);
  font-family: var(--font-mono);
}

.input:disabled {
  color: var(--text-faint);
  cursor: default;
}

@media (max-width: 700px) {
  .message-input {
    margin: 0 8px 8px 8px;
    padding: 0 8px;
  }

  .nick-prefix {
    display: none;
  }

  .emoji-autocomplete {
    left: 8px;
    right: 8px;
  }

  .slash-autocomplete {
    left: 8px;
    right: 8px;
  }

  .mention-autocomplete {
    left: 8px;
    right: 8px;
  }
}
</style>
