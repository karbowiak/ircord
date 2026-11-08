<script setup lang="ts">
const auth = useAuthStore()
</script>
<template>
  <!-- One delay for every tooltip, so moving along a toolbar shows the next label at once. -->
  <UiTooltipProvider>
    <AppWorkspace v-if="auth.status === 'signed-in'" />
    <LoginScreen v-else-if="auth.status === 'signed-out'" />
    <BootScreen
      v-else-if="auth.status === 'error'"
      title="Couldn’t reach IRCord"
      :detail="errorText(auth.session.error)"
      @retry="auth.session.refetch()"
    />
    <BootScreen v-else />
  </UiTooltipProvider>
</template>
