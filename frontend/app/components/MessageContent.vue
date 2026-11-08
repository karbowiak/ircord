<script setup lang="ts">
import { parseFormatting, splitCodeBlocks, stripFormatting } from '../../shared/utils/irc'
import { splitLists, type ListBlock } from '../../shared/utils/lists'
import { splitQuotes } from '../../shared/utils/quotes'
type Block =
  ListBlock | { kind: 'quote'; text: string } | { kind: 'code'; code: string; lang?: string }
const props = withDefaults(
  defineProps<{
    text: string
    /**
     * Only message bodies draw fenced code, quotes and Markdown lists as blocks. Everywhere else (topics,
     * lists, reaction chips) they stay inline, so a new caller is safe by default.
     */
    codeBlocks?: boolean
  }>(),
  { codeBlocks: false },
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

/**
 * Fenced blocks render as escaped preformatted text, `> ` lines as quotes and Markdown lists as
 * lists. The line breaks
 * around a block belong to the block, so the surrounding text drops one newline on that side.
 */
const blocks = computed(() => {
  const segments = splitCodeBlocks(props.text)
  return segments.flatMap((segment, index): Block[] => {
    if (segment.kind === 'code')
      return props.codeBlocks
        ? [{ kind: 'code', code: stripFormatting(segment.code), lang: segment.lang }]
        : [{ kind: 'text', text: `\x11${stripFormatting(segment.code)}\x11` }]
    let text = segment.raw
    if (!props.codeBlocks) return [{ kind: 'text', text }]
    if (segments[index - 1]?.kind === 'code') text = text.replace(/^\n/, '')
    if (segments[index + 1]?.kind === 'code') text = text.replace(/\n$/, '')
    return splitQuotes(text).flatMap((part): Block[] =>
      part.kind === 'quote' ? [part] : splitLists(part.text),
    )
  })
})
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
        :text="block.text" /><MessageRuns
        v-else
        :text="block.text"
    /></template>
  </span>
</template>
