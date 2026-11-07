<script setup lang="ts">
const auth = useAuthStore()
const mode = ref<'start' | 'key' | 'register'>('start')
const key = ref('')
const registrationKey = ref('')
const registrationInput = ref<HTMLInputElement | null>(null)
const error = ref('')
const saved = ref(false)
const copied = ref(false)
const keyInput = ref<HTMLTextAreaElement | null>(null)

async function create() {
  error.value = ''
  if (!auth.registration.data?.registrationEnabled) return
  if (auth.registration.data.registrationKeyRequired && mode.value !== 'register') {
    mode.value = 'register'
    await nextTick()
    registrationInput.value?.focus()
    return
  }
  try {
    await auth.register.mutateAsync(registrationKey.value || undefined)
    registrationKey.value = ''
  } catch (e) {
    error.value = errorText(e)
    await auth.registration.refetch()
    if (auth.registration.data?.registrationKeyRequired) mode.value = 'register'
  }
}
async function signIn() {
  error.value = ''
  try {
    await auth.login.mutateAsync(key.value)
  } catch (e) {
    error.value = errorText(e)
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(auth.newKey)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    error.value = 'Couldn’t copy. Select the key and copy it manually.'
  }
}
function download() {
  const blob = new Blob(
    [
      `IRCord account key\n\n${auth.newKey}\n\nThis key is the only way into your account. Keep it private.\n` +
        `Accounts unused for 90 days are deleted.\n`,
    ],
    { type: 'text/plain' },
  )
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = 'ircord-account-key.txt'
  link.click()
  URL.revokeObjectURL(link.href)
}
function leaveRegistration() {
  registrationKey.value = ''
  mode.value = 'start'
  error.value = ''
}
function useKey() {
  registrationKey.value = ''
  mode.value = 'key'
  error.value = ''
  nextTick(() => keyInput.value?.focus())
}
</script>
<template>
  <main class="login-screen">
    <div class="login-card">
      <span class="brand-mark"
        ><AppIcon
          name="brand"
          :size="26"
      /></span>

      <template v-if="auth.newKey">
        <h1>Save your account key</h1>
        <p class="login-copy">
          This key is your account. There’s no email or password, so if you lose it, nobody can
          recover your settings or networks.
        </p>
        <div
          class="account-key"
          aria-label="Your account key"
        >
          {{ auth.newKey }}
        </div>
        <div class="login-actions row">
          <button
            class="button secondary"
            @click="copy"
          >
            <AppIcon
              :name="copied ? 'check' : 'copy'"
              :size="15"
            />{{ copied ? 'Copied' : 'Copy' }}
          </button>
          <button
            class="button secondary"
            @click="download"
          >
            <AppIcon
              name="down"
              :size="15"
            />Download
          </button>
        </div>
        <label class="login-check"
          ><input
            v-model="saved"
            type="checkbox"
          />I’ve saved my key somewhere safe</label
        >
        <button
          class="button primary full-width"
          :disabled="!saved"
          @click="auth.confirmKey()"
        >
          Continue to IRCord
        </button>
      </template>

      <template v-else-if="mode === 'register'">
        <h1>Create an account</h1>
        <p class="login-copy">
          Registration is limited for now. Enter the registration key shared with you.
        </p>
        <form
          class="login-form"
          @submit.prevent="create"
        >
          <label
            class="field-label"
            for="registration-key"
            >Registration key</label
          >
          <input
            id="registration-key"
            ref="registrationInput"
            v-model="registrationKey"
            class="text-input"
            type="password"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            maxlength="256"
            required
            aria-describedby="registration-key-hint"
          />
          <p
            id="registration-key-hint"
            class="field-hint"
          >
            Only used to create an account. Your personal account key is generated next.
          </p>
          <button
            class="button primary full-width"
            :disabled="!registrationKey || auth.register.isLoading"
          >
            {{ auth.register.isLoading ? 'Creating…' : 'Create an account' }}
          </button>
        </form>
        <button
          class="text-button"
          @click="leaveRegistration"
        >
          Back
        </button>
      </template>

      <template v-else-if="mode === 'key'">
        <h1>Sign in with your key</h1>
        <form
          class="login-form"
          @submit.prevent="signIn"
        >
          <label
            class="field-label"
            for="account-key"
            >Account key</label
          >
          <textarea
            id="account-key"
            ref="keyInput"
            v-model="key"
            class="text-input account-key-input"
            rows="2"
            autocomplete="current-password"
            spellcheck="false"
            placeholder="ircord-XXXXX-XXXXX-…"
            required
            @keydown.enter.prevent="signIn"
          />
          <button
            class="button primary full-width"
            :disabled="!key.trim() || auth.login.isLoading"
          >
            {{ auth.login.isLoading ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>
        <button
          class="text-button"
          @click="mode = 'start'"
        >
          Back
        </button>
      </template>

      <template v-else>
        <h1>IRCord</h1>
        <p class="login-copy">
          Your IRC networks in one place, connected even when you’re not. No email or password: an
          account is a key that only you hold.
        </p>
        <div class="login-actions">
          <button
            v-if="auth.registration.data?.registrationEnabled"
            class="button primary full-width"
            :disabled="auth.register.isLoading"
            @click="create"
          >
            {{ auth.register.isLoading ? 'Creating…' : 'Create an account' }}
          </button>
          <p
            v-else-if="auth.registration.error"
            class="form-error"
            role="alert"
          >
            Registration availability could not be checked.
            <button
              class="text-button"
              @click="auth.registration.refetch()"
            >
              Try again
            </button>
          </p>
          <p
            v-else-if="auth.registration.data"
            class="login-note"
          >
            New registrations are currently closed.
          </p>
          <p
            v-else
            class="login-note"
            role="status"
          >
            Checking registration availability…
          </p>
          <button
            class="button secondary full-width"
            @click="useKey"
          >
            I have a key
          </button>
        </div>
        <p class="login-note">Accounts unused for 90 days are deleted with everything in them.</p>
      </template>

      <p
        v-if="error"
        class="form-error"
        role="alert"
      >
        {{ error }}
      </p>
    </div>
  </main>
</template>
