<script setup lang="ts" vapor>
definePageMeta({ keepalive: true })
const navigation = useConversationNavigation()
const actions = useWorkspaceActions()
</script>
<template>
  <DraftsPage
    @close="navigation.current()"
    @open="actions.resumeDraft"
  />
</template>
