<script setup lang="ts">
import type { MessageList } from '../../shared/utils/lists'
/** A Markdown list from a message body, nested lists included. Text renders as message runs. */
defineProps<{ list: MessageList }>()
</script>
<template>
  <component
    :is="list.ordered ? 'ol' : 'ul'"
    class="message-list"
    :start="list.ordered && list.start !== 1 ? list.start : undefined"
  >
    <li
      v-for="(item, index) in list.items"
      :key="index"
    >
      <MessageRuns :text="item.text" />
      <MessageListBlock
        v-for="(child, c) in item.children"
        :key="c"
        :list="child"
      />
    </li>
  </component>
</template>
