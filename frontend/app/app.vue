<script setup lang="ts">
const auth = useAuthStore()
</script>
<template>
  <AppWorkspace v-if="auth.status === 'signed-in'" />
  <LoginScreen v-else-if="auth.status === 'signed-out'" />
  <div
    v-else-if="auth.status === 'error'"
    class="boot-screen"
  >
    <AppIcon
      name="offline"
      :size="30"
    />
    <h1>Couldn’t reach IRCord</h1>
    <p>{{ errorText(auth.session.error) }}</p>
    <button
      class="button primary"
      @click="auth.session.refetch()"
    >
      <AppIcon
        name="refresh"
        :size="16"
      />Try again
    </button>
  </div>
  <div
    v-else
    class="boot-screen"
  >
    <span class="brand-mark"
      ><AppIcon
        name="brand"
        :size="26"
    /></span>
    <div class="loading-track"><span /></div>
  </div>
</template>
