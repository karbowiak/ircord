<script setup lang="ts">
import { parseFormatting, stripFormatting } from '../../shared/utils/irc'
import { readableOn } from '../../shared/utils/color'
const props = defineProps<{ text: string }>()
const media = useMediaStore()
const chat = useChatStore()
const appearance = useAppearanceStore()
watch(
  () => props.text,
  (text) => {
    if (/:[\w+-]{1,64}:/.test(text)) media.needCustom()
  },
  { immediate: true },
)
const plain = computed(() => stripFormatting(props.text))
const emojiOnly = computed(() => {
  if (
    /^(?:\p{Extended_Pictographic}|\p{Emoji_Presentation}|\p{Emoji_Modifier}|‍|️|\s){1,24}$/u.test(
      plain.value,
    )
  )
    return true
  if (!/^(?:\s*:[\w+-]{1,64}:\s*){1,3}$/.test(plain.value)) return false
  return [...plain.value.matchAll(/:([\w+-]{1,64}):/g)].every((match) =>
    media.byShortcode.has(match[1]!),
  )
})

/** IRC colours are adjusted so they stay readable on the current theme. */
function runStyle(run: ReturnType<typeof parseFormatting>[number]) {
  const background = run.bg?.startsWith('#') ? run.bg : appearance.tokens.main
  return {
    ...(run.fg
      ? { color: run.fg.startsWith('#') ? readableOn(run.fg, background, 4.5) : run.fg }
      : {}),
    ...(run.bg ? { backgroundColor: run.bg, borderRadius: '2px' } : {}),
  }
}
const ircLinks = useIrcLinks()
/** irc:// links open here instead of asking the operating system. */
function openIrc(event: MouseEvent, url: string) {
  if (!/^ircs?:\/\//i.test(url)) return
  event.preventDefault()
  void ircLinks.open(url)
}
/** Channel mentions refer to the network you're looking at. */
const joinedChannel = (text: string) =>
  chat.data?.conversations.find(
    (c) =>
      c.networkId === chat.networkId &&
      c.kind === 'channel' &&
      c.name.toLowerCase() === text.slice(1).toLowerCase(),
  )
function openChannel(text: string) {
  const existing = joinedChannel(text)
  if (existing) chat.activate(existing.id)
  else void chat.join.mutateAsync(text).catch((error) => chat.notify(errorText(error)))
}
const runs = computed(() =>
  parseFormatting(props.text).map((run) => ({
    run,
    style: runStyle(run),
    parts: run.text
      // A #channel starts a word and doesn't swallow trailing punctuation (“see #dev.”).
      .split(
        /(https?:\/\/[^\s]+|ircs?:\/\/[^\s]+|@[\w[\]\\^{}|`-]+|:[\w+-]{1,64}:|(?<![^\s(])#[^\s,#:]*[^\s,#:.!?)'"])/g,
      )
      .filter(Boolean)
      .map((text) => {
        const member = text.startsWith('@') ? chat.memberByNick(text.slice(1)) : undefined
        return {
          text,
          custom: text.startsWith(':') ? media.byShortcode.get(text.slice(1, -1)) : undefined,
          kind: /^(https?|ircs?):\/\//.test(text)
            ? 'link'
            : member
              ? 'mention'
              : /^#[^\s#]/.test(text)
                ? 'channel'
                : 'text',
          self: member?.id === 'you',
        }
      }),
  })),
)
</script>
<template>
  <span
    class="message-content"
    :class="{ 'emoji-only-message': emojiOnly }"
    ><span
      v-for="(segment, i) in runs"
      :key="i"
      :class="{
        'fmt-bold': segment.run.bold,
        'fmt-italic': segment.run.italic,
        'fmt-underline': segment.run.underline,
        'fmt-strike': segment.run.strike,
        'fmt-mono': segment.run.mono,
      }"
      :style="segment.style"
      ><template
        v-for="(part, index) in segment.parts"
        :key="index"
        ><CustomEmoji
          v-if="part.custom"
          :item="part.custom"
        /><a
          v-else-if="part.kind === 'link'"
          :href="part.text"
          target="_blank"
          rel="noopener noreferrer"
          @click="openIrc($event, part.text)"
          >{{ part.text }}</a
        ><span
          v-else-if="part.kind === 'mention'"
          class="inline-mention"
          :class="{ 'is-self': part.self }"
          >{{ part.text }}</span
        ><button
          v-else-if="part.kind === 'channel'"
          class="inline-channel"
          :title="joinedChannel(part.text) ? `Open ${part.text}` : `Join ${part.text}`"
          @click.stop="openChannel(part.text)"
        >
          {{ part.text }}</button
        ><template v-else>{{ part.text }}</template></template
      ></span
    ></span
  >
</template>
