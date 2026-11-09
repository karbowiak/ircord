<script setup lang="ts">
definePageMeta({ keepalive: true })
const navigation = useConversationNavigation()
const actions = useWorkspaceActions()
</script>
<template>
  <InboxPage
    @close="navigation.current()"
    @open="actions.reveal($event.conversationId, $event.messageId)"
  />
</template>
