<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import { stripFormatting } from '../../shared/utils/irc'
const props = defineProps<{ message: Message; grouped?: boolean; highlighted?: boolean }>()
const emit = defineEmits<{
  reply: [message: Message]
  profile: [id: string]
  image: [url: string, alt: string]
}>()
const chat = useChatStore()
const appearance = useAppearanceStore()
const library = useLibraryStore()
const nameStyle = useNameStyle()
const { time, full } = useTimeFormat()
const reacting = ref(false)
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
const quickReactions = ['👍', '❤️', '😂']
const menus = useMenus()
const authorMenu = (event: MouseEvent) => author.value && menus.member(event, author.value)
const row = ref<HTMLElement | null>(null)
/** The reaction picker opens from the toolbar button, or from the row (long-press menu). */
const reactAnchor = ref<HTMLElement | null>(null)
function openReactions(anchor: HTMLElement | null = row.value) {
  reactAnchor.value = anchor
  reacting.value = !!anchor
}
const messageMenu = (event: MouseEvent | HTMLElement) =>
  menus.message(event, props.message, () => openReactions())

async function copy() {
  try {
    await navigator.clipboard.writeText(stripFormatting(props.message.text))
    chat.notify('Copied to clipboard.')
  } catch {
    chat.notify('Select the message text to copy it.')
  }
}
function react(item: EmojiItem) {
  reacting.value = false
  library.useEmoji(item)
  void chat.react(props.message, emojiText(item, library.library.skinTone))
}
</script>
<template>
  <article
    class="message-row"
    :class="{
      'is-grouped': grouped,
      'is-highlighted': highlighted,
      'is-mention': mentionsMe,
      'is-pending': message.delivery === 'pending',
      'is-failed': message.delivery === 'failed',
      'is-mine': message.senderId === 'you',
      'is-action': action,
      'is-notice': message.kind === 'notice',
      'show-actions': reacting,
    }"
    ref="row"
    :aria-label="`${author?.displayName}, ${time(message.createdAt)}`"
    @contextmenu="messageMenu($event)"
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
        {{ author?.nick ?? author?.displayName }}</button
      ><span class="message-body">
        <span
          v-if="showText"
          class="message-text"
          ><MessageContent :text="message.text"
        /></span>
        <GifMessage
          v-else-if="message.gif"
          :gif="message.gif"
          @image="(url, alt) => emit('image', url, alt)"
        />
      </span>
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
        <UserAvatar :member="author" />
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
          v-if="showText"
          class="message-text"
        >
          <span
            v-if="action"
            class="action-nick"
            :style="nameStyle(author)"
            >{{ author?.nick }}
          </span>
          <MessageContent :text="message.text" />
        </p>
        <GifMessage
          v-else-if="message.gif"
          :gif="message.gif"
          @image="(url, alt) => emit('image', url, alt)"
        />
        <MessageExtras
          :message="message"
          @image="(url, alt) => emit('image', url, alt)"
        />
      </div>
    </div>

    <MessageExtras
      v-if="irc"
      :message="message"
      @image="(url, alt) => emit('image', url, alt)"
    />

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
      v-if="!message.delivery"
      class="message-actions"
    >
      <button
        v-for="emoji in quickReactions"
        :key="emoji"
        class="icon-button quick-reaction"
        :aria-label="`React with ${emoji}`"
        @click="chat.react(message, emoji)"
      >
        {{ emoji }}
      </button>
      <span class="action-divider" />
      <button
        ref="reactButton"
        class="icon-button"
        aria-label="Add reaction"
        title="Add reaction"
        :aria-expanded="reacting"
        @click="reacting ? (reacting = false) : openReactions(reactButton)"
      >
        <AppIcon
          name="smilePlus"
          :size="17"
        /></button
      ><button
        class="icon-button"
        aria-label="Reply to message"
        title="Reply"
        @click="emit('reply', message)"
      >
        <AppIcon
          name="reply"
          :size="17"
        /></button
      ><button
        class="icon-button"
        aria-label="Copy message"
        title="Copy text"
        @click="copy"
      >
        <AppIcon
          name="copy"
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
