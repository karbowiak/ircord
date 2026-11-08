<script setup lang="ts">
import type { MessageBlock } from '../../shared/utils/blocks'
/** A quote from a message body. Its contents keep formatting, code blocks and Markdown lists. */
defineProps<{ blocks: MessageBlock[] }>()
</script>
<template>
  <blockquote
    class="message-quote block my-0.5 mx-0 pl-3 pr-0 py-0 border-l-4 border-l-faint [border-left-style:solid] rounded-[4px] [&>.message-list:first-child]:m-0 [&>.message-list:last-child]:m-0"
  >
    <template
      v-for="(block, b) in blocks"
      :key="b"
      ><CodeBlock
        v-if="block.kind === 'code'"
        :code="block.code"
        :lang="block.lang" /><MessageListBlock
        v-else-if="block.kind === 'list'"
        :list="block.list" /><MessageRuns
        v-else-if="block.kind === 'text'"
        :text="block.text"
    /></template>
  </blockquote>
</template>
