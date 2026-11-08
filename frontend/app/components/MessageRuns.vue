<script setup lang="ts">
import { parseFormatting } from '../../shared/utils/irc'
import { readableOn } from '../../shared/utils/color'
import { isGroupMention } from '../../shared/group-mentions'
/** One stretch of message text: IRC formatting, links, mentions, emoji and #channels. */
const props = defineProps<{ text: string }>()
const media = useMediaStore()
const chat = useChatStore()
const appearance = useAppearanceStore()

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
/** Links, mentions, emoji and #channels become interactive; text inside code stays literal. */
const runs = computed(() =>
  parseFormatting(props.text).map((run) => ({
    run,
    style: runStyle(run),
    parts: run.mono
      ? [{ text: run.text, kind: 'text', custom: undefined, self: false }]
      : run.text
          // A #channel starts a word and doesn't swallow trailing punctuation (“see #dev.”).
          .split(
            /(https?:\/\/[^\s]+|ircs?:\/\/[^\s]+|@[\w[\]\\^{}|`-]+|:[\w+-]{1,64}:|(?<![^\s(])#[^\s,#:]*[^\s,#:.!?)'"])/g,
          )
          .filter(Boolean)
          .map((text) => {
            const member = text.startsWith('@') ? chat.memberByNick(text.slice(1)) : undefined
            // @here, @everyone, @operator…: IRCord group mentions (a member's own nick wins).
            const group = !member && text.startsWith('@') && isGroupMention(text)
            return {
              text,
              custom: text.startsWith(':') ? media.byShortcode.get(text.slice(1, -1)) : undefined,
              kind: /^(https?|ircs?):\/\//.test(text)
                ? 'link'
                : member || group
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
  >
</template>
