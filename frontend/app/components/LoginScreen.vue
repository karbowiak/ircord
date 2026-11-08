<script setup lang="ts">
/**
 * Sign in or create an account with a username and password. The username is also your nick
 * and account on irc.ircord.com, so it's checked against the network while you type.
 */
const auth = useAuthStore()
const mode = ref<'signin' | 'signup'>('signin')
const username = ref('')
const password = ref('')
const displayName = ref('')
const registrationKey = ref('')
const error = ref('')
const first = ref<HTMLInputElement | null>(null)

const registration = computed(() => auth.registration.data)
const busy = computed(() => auth.login.isLoading || auth.signup.isLoading)

function go(next: typeof mode.value) {
  mode.value = next
  error.value = ''
  password.value = ''
  registrationKey.value = ''
  nextTick(() => first.value?.focus())
}

const { check, checking } = useUsernameCheck(username, () => mode.value === 'signup')

const canSignUp = computed(
  () =>
    !!username.value.trim() &&
    !checking.value &&
    check.value?.available !== false &&
    password.value.length >= 8 &&
    (!registration.value?.registrationKeyRequired || !!registrationKey.value.trim()),
)

async function attempt(action: () => Promise<unknown>) {
  error.value = ''
  try {
    await action()
  } catch (e) {
    error.value = errorText(e)
  }
}
const signIn = () =>
  attempt(() =>
    auth.login.mutateAsync({ username: username.value.trim(), password: password.value }),
  )
const signUp = () =>
  attempt(() =>
    auth.signup.mutateAsync({
      username: username.value.trim(),
      password: password.value,
      ...(displayName.value.trim() ? { displayName: displayName.value.trim() } : {}),
      ...(registrationKey.value.trim() ? { registrationKey: registrationKey.value.trim() } : {}),
    }),
  )
</script>
<template>
  <main class="login-screen">
    <div class="login-card">
      <span class="brand-mark"
        ><AppIcon
          name="brand"
          :size="26"
      /></span>

      <template v-if="mode === 'signup'">
        <h1>Create your account</h1>
        <p class="login-copy">
          One account for IRCord and irc.ircord.com. You’re logged in there from the start, and the
          same username and password work in any IRC client.
        </p>
        <form
          class="login-form"
          @submit.prevent="signUp"
        >
          <label
            class="field-label"
            for="signup-username"
            >Username</label
          >
          <input
            id="signup-username"
            ref="first"
            v-model="username"
            class="text-input"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            maxlength="30"
            required
            :aria-invalid="check?.available === false"
            aria-describedby="signup-username-hint"
          />
          <p
            id="signup-username-hint"
            class="field-hint"
            :class="{ 'danger-text': check?.available === false }"
            aria-live="polite"
          >
            <template v-if="check?.available === false">{{ check.reason }}</template>
            <template v-else-if="check?.available"
              ><AppIcon
                name="check"
                :size="13"
              />
              {{ username.trim() }} is available</template
            >
            <template v-else-if="checking">Checking…</template>
            <template v-else>Also your nick. You can set a display name later.</template>
          </p>
          <label
            class="field-label"
            for="signup-display-name"
            >Display name <small>(optional)</small></label
          >
          <input
            id="signup-display-name"
            v-model="displayName"
            class="text-input"
            maxlength="80"
            autocomplete="nickname"
            aria-describedby="signup-display-name-hint"
          />
          <p
            id="signup-display-name-hint"
            class="field-hint"
          >
            How you appear in chat. Spaces are fine; you can change it later.
          </p>
          <label
            class="field-label"
            for="signup-password"
            >Password</label
          >
          <input
            id="signup-password"
            v-model="password"
            class="text-input"
            type="password"
            autocomplete="new-password"
            minlength="8"
            maxlength="128"
            required
            aria-describedby="signup-password-hint"
          />
          <p
            id="signup-password-hint"
            class="field-hint"
          >
            At least 8 characters, no spaces.
          </p>
          <template v-if="registration?.registrationKeyRequired">
            <label
              class="field-label"
              for="registration-key"
              >Registration key</label
            >
            <input
              id="registration-key"
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
              Registration is limited for now. Enter the key shared with you.
            </p>
          </template>
          <button
            class="button primary full-width"
            :disabled="!canSignUp || busy"
          >
            {{ auth.signup.isLoading ? 'Creating your account…' : 'Create account' }}
          </button>
        </form>
        <p class="login-switch">
          Already have an account?
          <button
            class="text-button"
            @click="go('signin')"
          >
            Sign in
          </button>
        </p>
      </template>

      <template v-else>
        <h1>Sign in to IRCord</h1>
        <p class="login-copy">Your IRC networks in one place, connected even when you’re not.</p>
        <form
          class="login-form"
          @submit.prevent="signIn"
        >
          <label
            class="field-label"
            for="signin-username"
            >Username</label
          >
          <input
            id="signin-username"
            ref="first"
            v-model="username"
            class="text-input"
            autocomplete="username"
            autocapitalize="none"
            spellcheck="false"
            required
          />
          <label
            class="field-label"
            for="signin-password"
            >Password</label
          >
          <input
            id="signin-password"
            v-model="password"
            class="text-input"
            type="password"
            autocomplete="current-password"
            required
          />
          <button
            class="button primary full-width"
            :disabled="!username.trim() || !password || busy"
          >
            {{ auth.login.isLoading ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>
        <p
          v-if="registration?.registrationEnabled"
          class="login-switch"
        >
          New here?
          <button
            class="text-button"
            @click="go('signup')"
          >
            Create an account
          </button>
        </p>
        <p
          v-else-if="auth.registration.error"
          class="form-error"
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
          v-else-if="registration"
          class="login-note"
        >
          New registrations are currently closed.
        </p>
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
