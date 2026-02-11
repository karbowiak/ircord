<template>
  <div class="message" :class="{ 'is-action': isAction, 'is-mentioned-self': isMentioningSelf && !isOwnMessage }">
    <div class="hover-actions" :class="{ visible: isActionBarVisible }">
      <button type="button" class="action-btn" @click.stop="toggleReactionPicker">😊</button>
      <button v-if="isOwnMessage" type="button" class="action-btn" @click.stop="startEditing">✎</button>
      <button type="button" class="action-btn" @click.stop="onReply">↩</button>
    </div>

    <div v-if="reactionPickerOpen" class="reaction-picker" @click.stop>
      <button
        v-for="emoji in appStore.recentReactions"
        :key="`recent-${emoji}`"
        type="button"
        class="reaction-btn"
        @click="onReact(emoji)"
      >
        {{ emoji }}
      </button>
      <button
        v-for="emoji in fallbackReactions"
        :key="`fallback-${emoji}`"
        type="button"
        class="reaction-btn"
        @click="onReact(emoji)"
      >
        {{ emoji }}
      </button>
    </div>

    <div class="avatar">{{ author.avatar }}</div>
    <div class="content">
      <div class="header">
        <span class="username" :class="nickClass">{{ author.username }}</span>
        <span class="timestamp">{{ timestamp }}</span>
        <span v-if="replyPreview" class="reply-inline" :title="replyPreview">{{ replyPreview }}</span>
      </div>
      <div v-if="isEditing" class="edit-wrap">
        <input
          ref="editInputEl"
          v-model="editDraft"
          class="edit-input"
          maxlength="2000"
          @keydown.enter.prevent="submitEdit"
          @keydown.esc.prevent="cancelEdit"
          @blur="submitEdit"
        >
      </div>

      <div v-else class="text">
        <template v-for="(segment, idx) in textSegments" :key="`seg-${idx}`">
          <a
            v-if="segment.type === 'link'"
            :href="segment.value"
            class="inline-link"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ segment.value }}
          </a>
          <img
            v-else-if="segment.type === 'emoji-image'"
            :src="segment.src"
            :alt="segment.label"
            :title="segment.label"
            class="inline-emoji-image"
            loading="lazy"
            decoding="async"
          >
          <span
            v-else-if="segment.type === 'emoji-char'"
            class="inline-emoji-char"
            :title="segment.label"
            :aria-label="segment.label"
          >
            {{ segment.value }}
          </span>
          <span v-else v-html="renderTextWithMentions(segment.value)"></span>
        </template>
      </div>

      <div v-if="reactions.length" class="reaction-row">
        <button
          v-for="emoji in reactions"
          :key="emoji"
          type="button"
          class="reaction-chip"
          @click="onReact(emoji)"
        >
          {{ emoji }}
        </button>
      </div>

      <AnimatedMediaPreviewList
        :media-urls="mediaPreviewUrls"
        :media-scale-percent="appStore.mediaScalePercent"
        :gif-autoplay="appStore.gifAutoplay"
      />

      <YouTubeEmbedList
        :embed-urls="youtubeEmbedUrls"
        :media-scale-percent="appStore.mediaScalePercent"
      />

      <ResolvingMediaList
        :urls="resolvingPageUrls"
        :media-scale-percent="appStore.mediaScalePercent"
      />

      <LinkPreviewCardList
        :cards="linkPreviewCards"
        :media-scale-percent="appStore.mediaScalePercent"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { User } from '~/types'
import { findEmojiByShortcode } from '~/composables/useEmojiData'
import { useMessageEmbeds } from '~/composables/useMessageEmbeds'
import AnimatedMediaPreviewList from '~/components/chat/embeds/AnimatedMediaPreviewList.vue'
import YouTubeEmbedList from '~/components/chat/embeds/YouTubeEmbedList.vue'
import ResolvingMediaList from '~/components/chat/embeds/ResolvingMediaList.vue'
import LinkPreviewCardList from '~/components/chat/embeds/LinkPreviewCardList.vue'

interface Props {
  messageId: string
  author: User
  content: string
  timestamp: string
  replyTo?: {
    messageId: string
    authorUsername: string
    content: string
  }
  isOwnMessage?: boolean
  reactions?: string[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  reply: [messageId: string]
  react: [{ messageId: string, emoji: string }]
  edit: [{ messageId: string, content: string }]
}>()

const appStore = useAppStore()
const { mediaPreviewUrls, youtubeEmbedUrls, resolvingPageUrls, linkPreviewCards } = useMessageEmbeds(toRef(props, 'content'))
const isEditing = ref(false)
const reactionPickerOpen = ref(false)
const editDraft = ref('')
const editInputEl = ref<HTMLInputElement>()
const fallbackReactions = ['👍', '❤️', '😂']
const reactions = computed(() => props.reactions || [])
const isOwnMessage = computed(() => Boolean(props.isOwnMessage))
const isActionBarVisible = computed(() => !isEditing.value)
const replyPreview = computed(() => {
  if (!props.replyTo) return ''
  return `Replying to ${props.replyTo.authorUsername}: ${props.replyTo.content}`
})

const isAction = computed(() => /^\/me\s+|^ACTION\s+/i.test(props.content))
const actionContent = computed(() => {
  if (!isAction.value) return props.content
  return props.content.replace(/^\/me\s+/i, '').replace(/^ACTION\s+/i, '')
})
const selfNick = computed(() => (appStore.activeServerStatus?.nick || appStore.currentUser.username).toLowerCase())
const isMentioningSelf = computed(() => {
  const regex = /(^|\s)@([A-Za-z0-9_\-\[\]\\`^{}|]{1,32})\b/g
  let match: RegExpExecArray | null
  while ((match = regex.exec(props.content)) !== null) {
    if ((match[2] || '').toLowerCase() === selfNick.value) {
      return true
    }
  }
  return false
})

const nickClass = computed(() => {
  // Color nicks based on hash of username for variety
  const hash = props.author.username.split('').reduce((acc, char) => {
    return char.charCodeAt(0) + ((acc << 5) - acc)
  }, 0)
  const colors = ['nick-blue', 'nick-green', 'nick-orange', 'nick-purple', 'nick-cyan', 'nick-pink']
  return colors[Math.abs(hash) % colors.length]
})

type Segment = {
  type: 'text' | 'link' | 'emoji-char'
  value: string
  label?: string
} | {
  type: 'emoji-image'
  src: string
  label: string
}

const emojiRegex = /:([a-z0-9_+-]+):/gi

const textSegments = computed<Segment[]>(() => {
  const content = actionContent.value
  const urlRegex = /(https?:\/\/[^\s]+)/g
  const matches = Array.from(content.matchAll(urlRegex))
  if (!matches.length) return applyEmojiShortcodes(content)

  const segments: Segment[] = []
  let cursor = 0

  for (const match of matches) {
    const full = match[0]
    const index = match.index ?? 0

    if (index > cursor) {
      segments.push(...applyEmojiShortcodes(content.slice(cursor, index)))
    }

    segments.push({ type: 'link', value: full })
    cursor = index + full.length
  }

  if (cursor < content.length) {
    segments.push(...applyEmojiShortcodes(content.slice(cursor)))
  }

  return segments
})

function applyEmojiShortcodes(input: string): Segment[] {
  const matches = Array.from(input.matchAll(emojiRegex))
  if (!matches.length) return [{ type: 'text', value: input }]

  const segments: Segment[] = []
  let cursor = 0

  for (const match of matches) {
    const full = match[0]
    const shortcode = match[1]
    const index = match.index ?? 0

    if (index > cursor) {
      segments.push({ type: 'text', value: input.slice(cursor, index) })
    }

    const emoji = findEmojiByShortcode(shortcode)
    if (emoji?.imageUrl) {
      segments.push({
        type: 'emoji-image',
        src: emoji.imageUrl,
        label: `:${emoji.shortcode}:`,
      })
    } else if (emoji?.char) {
      segments.push({
        type: 'emoji-char',
        value: emoji.char,
        label: `:${emoji.shortcode}:`,
      })
    } else {
      segments.push({ type: 'text', value: full })
    }

    cursor = index + full.length
  }

  if (cursor < input.length) {
    segments.push({ type: 'text', value: input.slice(cursor) })
  }

  return segments
}

function renderTextWithMentions(input: string): string {
  const escaped = escapeHtml(input)
  return escaped.replace(/(^|\s)@([A-Za-z0-9_\-\[\]\\`^{}|]{1,32})\b/g, (_full, prefix: string, nick: string) => {
    const isSelf = nick.toLowerCase() === selfNick.value
    return `${prefix}<span class="mention${isSelf ? ' mention-self' : ''}">@${nick}</span>`
  })
}

function escapeHtml(input: string): string {
  return input
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function onReply() {
  emit('reply', props.messageId)
}

function onReact(emoji: string) {
  reactionPickerOpen.value = false
  emit('react', { messageId: props.messageId, emoji })
}

function toggleReactionPicker() {
  reactionPickerOpen.value = !reactionPickerOpen.value
}

function startEditing() {
  if (!isOwnMessage.value) return
  isEditing.value = true
  reactionPickerOpen.value = false
  editDraft.value = props.content
  nextTick(() => {
    editInputEl.value?.focus()
    editInputEl.value?.select()
  })
}

function cancelEdit() {
  isEditing.value = false
  editDraft.value = ''
}

function submitEdit() {
  if (!isEditing.value) return
  const text = editDraft.value.trim()
  if (!text) {
    cancelEdit()
    return
  }
  emit('edit', { messageId: props.messageId, content: text })
  isEditing.value = false
}

function onGlobalPointerDown(event: MouseEvent) {
  if (!reactionPickerOpen.value) return
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.closest('.reaction-picker') || target.closest('.hover-actions')) return
  reactionPickerOpen.value = false
}

onMounted(() => {
  window.addEventListener('mousedown', onGlobalPointerDown, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('mousedown', onGlobalPointerDown, { capture: true })
})
</script>

<style scoped>
.message {
  position: relative;
  display: flex;
  padding: 2px 16px 2px 72px;
  margin-top: 16px;
  transition: background-color 0.1s ease;
}

.hover-actions {
  position: absolute;
  top: -12px;
  right: 20px;
  display: none;
  gap: 4px;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(25, 27, 33, 0.95);
  z-index: 5;
}

.message:hover .hover-actions.visible {
  display: flex;
}

.action-btn {
  border: 0;
  border-radius: 6px;
  width: 24px;
  height: 24px;
  background: transparent;
  color: var(--text-body);
  font-size: 12px;
  cursor: pointer;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.reaction-picker {
  position: absolute;
  top: 16px;
  right: 20px;
  z-index: 6;
  display: flex;
  gap: 4px;
  padding: 6px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(22, 24, 29, 0.98);
}

.reaction-btn {
  border: 0;
  border-radius: 6px;
  width: 28px;
  height: 28px;
  background: rgba(255, 255, 255, 0.06);
  font-size: 15px;
  cursor: pointer;
}

.reaction-btn:hover {
  background: rgba(88, 101, 242, 0.22);
}

.reaction-row {
  margin-top: 8px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.reaction-chip {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 11px;
  padding: 3px 8px;
  cursor: pointer;
}

.reaction-chip:hover {
  background: rgba(88, 101, 242, 0.2);
}

.edit-wrap {
  margin-top: 2px;
}

.edit-input {
  width: min(100%, 720px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.22);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 13px;
  padding: 7px 10px;
  outline: none;
}

.edit-input:focus {
  border-color: rgba(88, 101, 242, 0.72);
}

.message:first-child {
  margin-top: 0;
}

.message:hover {
  background-color: rgba(0, 0, 0, 0.06);
}

.message.is-mentioned-self {
  background: linear-gradient(90deg, rgba(255, 196, 74, 0.12), rgba(255, 196, 74, 0));
  border-left: 2px solid rgba(255, 196, 74, 0.42);
}

.avatar {
  position: absolute;
  left: 16px;
  top: 2px;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: var(--bg-active);
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-mono);
  font-size: 16px;
  font-weight: 600;
  color: var(--text-body);
  flex-shrink: 0;
}

.content {
  flex: 1;
  min-width: 0;
}

.header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 2px;
}

.username {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.username:hover {
  text-decoration: underline;
}

/* Nick colors - IRC style */
.nick-blue { color: #7289da; }
.nick-green { color: #43b581; }
.nick-orange { color: #f0a020; }
.nick-purple { color: #b48eff; }
.nick-cyan { color: #00d4aa; }
.nick-pink { color: #f47fff; }

.timestamp {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
}

.reply-inline {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  max-width: min(62vw, 520px);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.text {
  font-family: var(--font-mono);
  font-size: 14px;
  color: var(--text-body);
  line-height: 1.4;
  word-wrap: break-word;
  white-space: pre-wrap;
}

.inline-link {
  color: #87c7ff;
  text-decoration: none;
}

:deep(.mention) {
  color: #9fb7ff;
  font-weight: 600;
}

:deep(.mention.mention-self) {
  color: #ffd269;
}

.inline-link:hover {
  text-decoration: underline;
}

.inline-emoji-char {
  display: inline-block;
  line-height: 1;
  font-size: 18px;
  vertical-align: -3px;
}

.inline-emoji-image {
  width: 20px;
  height: 20px;
  object-fit: contain;
  vertical-align: -4px;
}

.is-action .text {
  color: var(--text-muted);
  font-style: italic;
}

@media (max-width: 600px) {
  .message {
    padding-left: 16px;
  }

  .avatar {
    display: none;
  }

}
</style>
