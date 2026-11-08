<script setup lang="ts">
import { messageBlocks, type MessageBlock } from '../../shared/utils/blocks'
import { parseFormatting, splitCodeBlocks, stripFormatting } from '../../shared/utils/irc'
const props = withDefaults(
  defineProps<{
    text: string
    /**
     * Only message bodies draw fenced code, quotes and Markdown lists as blocks. Everywhere else
     * (topics, lists, reaction chips) they stay inline, so a new caller is safe by default.
     */
    codeBlocks?: boolean
    /** The text follows other content on its first line (an action's nick). */
    continuesLine?: boolean
  }>(),
  { codeBlocks: false, continuesLine: false },
)
const media = useMediaStore()
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
    blocks.value.some(
      (block) => block.kind !== 'text' || parseFormatting(block.text).some((run) => run.mono),
    )
  )
    return false
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

/** Message bodies split into blocks; elsewhere fenced code shows inline as monospace text. */
const blocks = computed((): MessageBlock[] =>
  props.codeBlocks
    ? messageBlocks(props.text, { continuesLine: props.continuesLine })
    : splitCodeBlocks(props.text).map((segment) => ({
        kind: 'text',
        text: segment.kind === 'code' ? `\x11${stripFormatting(segment.code)}\x11` : segment.raw,
      })),
)
</script>
<template>
  <span
    class="message-content"
    :class="{ 'emoji-only-message': emojiOnly }"
    ><template
      v-for="(block, b) in blocks"
      :key="b"
      ><CodeBlock
        v-if="block.kind === 'code'"
        :code="block.code"
        :lang="block.lang" /><MessageListBlock
        v-else-if="block.kind === 'list'"
        :list="block.list" /><MessageQuoteBlock
        v-else-if="block.kind === 'quote'"
        :blocks="block.blocks" /><MessageRuns
        v-else
        :text="block.text"
    /></template>
  </span>
</template>
