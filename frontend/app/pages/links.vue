<script setup lang="ts">
definePageMeta({ keepalive: true })
const route = useRoute()
const navigation = useConversationNavigation()
const actions = useWorkspaceActions()
const channel = computed(() =>
  typeof route.query.channel === 'string' ? route.query.channel : undefined,
)
</script>
<template>
  <LinksPage
    :conversation-id="channel"
    @close="navigation.current()"
    @open="actions.reveal($event.conversationId, $event.messageId)"
  />
</template>
