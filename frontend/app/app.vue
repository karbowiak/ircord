<script setup lang="ts">
const auth = useAuthStore()
</script>
<template>
  <AppWorkspace v-if="auth.status === 'signed-in'" />
  <LoginScreen v-else-if="auth.status === 'signed-out'" />
  <div
    v-else-if="auth.status === 'error'"
    class="boot-screen fixed inset-0 flex items-center justify-center flex-col gap-4 bg-background text-body text-center p-6"
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
    class="boot-screen fixed inset-0 flex items-center justify-center flex-col gap-4 bg-background text-body text-center p-6"
  >
    <BrandMark class="brand-mark block w-12 h-12 flex-none" />
    <div class="loading-track h-[3px] w-30 bg-border overflow-hidden rounded-[3px]"><span /></div>
  </div>
</template>
