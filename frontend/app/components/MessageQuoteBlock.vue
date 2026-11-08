<script setup lang="ts">
import type { MessageBlock } from '../../shared/utils/blocks'
/** A quote from a message body. Its contents keep formatting, code blocks and Markdown lists. */
defineProps<{ blocks: MessageBlock[] }>()
</script>
<template>
  <blockquote class="message-quote">
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
