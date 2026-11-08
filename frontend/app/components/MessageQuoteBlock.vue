<script setup lang="ts">
import { splitLists } from '../../shared/utils/lists'
/** A quote from a message body. Its text keeps message formatting and Markdown lists. */
const props = defineProps<{ text: string }>()
const blocks = computed(() => splitLists(props.text))
</script>
<template>
  <blockquote class="message-quote">
    <template
      v-for="(block, b) in blocks"
      :key="b"
      ><MessageListBlock
        v-if="block.kind === 'list'"
        :list="block.list" /><MessageRuns
        v-else
        :text="block.text"
    /></template>
  </blockquote>
</template>
