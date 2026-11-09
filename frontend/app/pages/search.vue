<script setup lang="ts">
definePageMeta({ keepalive: true })
const navigation = useConversationNavigation()
const actions = useWorkspaceActions()
</script>
<template>
  <SearchPage
    @close="navigation.current()"
    @jump="actions.reveal"
  />
</template>
