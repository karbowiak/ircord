<script setup lang="ts">
/**
 * Asks an account from before usernames (#64) to choose a username and password. The key keeps
 * working until keys are retired, so "Not now" only hides this for the current visit.
 */
const emit = defineEmits<{ close: [] }>()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const error = ref('')
const { check, checking } = useUsernameCheck(username)
const ready = computed(() => check.value?.available === true && password.value.length >= 8)

async function save() {
  error.value = ''
  try {
    await auth.setCredentials.mutateAsync({
      username: username.value.trim(),
      password: password.value,
    })
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  }
}
</script>
<template>
  <AppDialog
    title="Choose a username and password"
    @close="emit('close')"
  >
    <form
      class="credentials-form"
      @submit.prevent="save"
    >
      <p class="field-hint">
        Account keys are going away. Your username is also your account on irc.ircord.com, and the
        same login works in any IRC client. Your networks, history and settings stay as they are.
      </p>
      <label
        class="field-label"
        for="credentials-username"
        >Username</label
      >
      <input
        id="credentials-username"
        v-model="username"
        class="text-input"
        autocomplete="username"
        autocapitalize="none"
        spellcheck="false"
        maxlength="30"
        required
        :aria-invalid="check?.available === false"
        aria-describedby="credentials-username-hint"
      />
      <p
        id="credentials-username-hint"
        class="field-hint"
        :class="{ 'danger-text': check?.available === false }"
        aria-live="polite"
      >
        <template v-if="check?.available === false">{{ check.reason }}</template>
        <template v-else-if="check?.available">{{ username.trim() }} is available</template>
        <template v-else-if="checking">Checking…</template>
        <template v-else>Also your nick on irc.ircord.com.</template>
      </p>
      <label
        class="field-label"
        for="credentials-password"
        >Password</label
      >
      <input
        id="credentials-password"
        v-model="password"
        class="text-input"
        type="password"
        autocomplete="new-password"
        minlength="8"
        maxlength="128"
        required
      />
      <p class="field-hint">At least 8 characters.</p>
      <p
        v-if="error"
        class="form-error"
        role="alert"
      >
        {{ error }}
      </p>
      <div class="form-actions">
        <button
          type="button"
          class="button secondary"
          @click="emit('close')"
        >
          Not now
        </button>
        <button
          class="button primary"
          :disabled="!ready || auth.setCredentials.isLoading"
        >
          {{ auth.setCredentials.isLoading ? 'Saving…' : 'Save' }}
        </button>
      </div>
    </form>
  </AppDialog>
</template>
