<template>
  <div class="message-input-wrap">
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

    <div class="message-input">
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

      <span class="nick-prefix">{{ appStore.currentUser.username }}&gt;</span>
      <input
        ref="inputEl"
        v-model="messageText"
        type="text"
        :placeholder="placeholder"
        class="input"
        @keydown="onInputKeydown"
        @input="updateEmojiAutocomplete"
        @click="updateEmojiAutocomplete"
      >

      <div class="right-controls">
        <button
          type="button"
          class="icon-button"
          title="Open emoji picker"
          @click="toggleEmojiPicker"
        >
          :)
        </button>

        <button
          type="button"
          class="icon-button"
          title="Open GIF picker"
          @click="toggleGifPicker"
        >
          GIF
        </button>

        <button
          type="button"
          class="icon-button send"
          title="Send"
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
const isGifPickerOpen = ref(false)
const isEmojiPickerOpen = ref(false)
const activeEmojiSuggestionIndex = ref(0)
const emojiAutocompleteOpen = ref(false)
const emojiAutocompleteQuery = ref('')
const emojiReplaceRange = ref<{ start: number, end: number } | null>(null)

interface EmojiSuggestion {
  shortcode: string
  char?: string
  imageUrl?: string
  keywords: string[]
}

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
  const channel = appStore.activeChannel
  if (!channel) return 'Type a message...'
  if ('name' in channel) {
    return `Message #${channel.name}... (or /command)`
  }
  return `Message ${channel.recipient.username}...`
})

function sendMessage() {
  const normalizedText = normalizeEmoticonsInText(messageText.value)

  if (appStore.sendMockMessage(normalizedText)) {
    messageText.value = ''
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    nextTick(() => inputEl.value?.focus())
  }
}

function toggleGifPicker() {
  closeEmojiAutocomplete()
  isEmojiPickerOpen.value = false
  isGifPickerOpen.value = !isGifPickerOpen.value
}

function toggleEmojiPicker() {
  closeEmojiAutocomplete()
  isGifPickerOpen.value = false
  isEmojiPickerOpen.value = !isEmojiPickerOpen.value
}

function onGifSelect(url: string) {
  if (appStore.sendMockMessage(url)) {
    isGifPickerOpen.value = false
    closeEmojiAutocomplete()
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
  nextTick(() => inputEl.value?.focus())
}

function closeEmojiAutocomplete() {
  emojiAutocompleteOpen.value = false
  emojiAutocompleteQuery.value = ''
  activeEmojiSuggestionIndex.value = 0
  emojiReplaceRange.value = null
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
    sendMessage()
  }
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

  if (lower === 'escape' && (isGifPickerOpen.value || isEmojiPickerOpen.value)) {
    isGifPickerOpen.value = false
    isEmojiPickerOpen.value = false
    closeEmojiAutocomplete()
    nextTick(() => inputEl.value?.focus())
  }
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKeydown, { capture: true })
})
</script>

<style scoped>
.message-input-wrap {
  position: relative;
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
}
</style>
