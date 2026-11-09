<script setup lang="ts">
const auth = useAuthStore()
const { t } = useI18n()
</script>
<template>
  <AppWorkspace v-if="auth.status === 'signed-in'"><slot /></AppWorkspace>
  <LoginScreen v-else-if="auth.status === 'signed-out'" />
  <BootScreen
    v-else-if="auth.status === 'error'"
    :title="$t('app.couldNotReach')"
    :detail="errorText(auth.session.error, (key) => t(key))"
    @retry="auth.session.refetch()"
  />
  <BootScreen v-else />
</template>
