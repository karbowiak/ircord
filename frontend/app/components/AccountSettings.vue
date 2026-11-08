<script setup lang="ts">
/**
 * Settings → Account: your username and password, sign-out and deletion. Accounts from before
 * usernames (#64) still show their key and can choose a username here.
 */
const auth = useAuthStore()
const { confirm } = usePrompt()
const username = computed(() => auth.account?.username)
const choosing = ref(false)

const current = ref('')
const next = ref('')
const passwordError = ref('')
const passwordSaved = ref(false)
async function changePassword() {
  passwordError.value = ''
  passwordSaved.value = false
  try {
    await auth.changePassword.mutateAsync({ current: current.value, password: next.value })
    current.value = next.value = ''
    passwordSaved.value = true
  } catch (e) {
    passwordError.value = errorText(e)
  }
}

const rotatedKey = ref('')
async function rotateKey() {
  if (
    await confirm({
      title: 'Create a new account key?',
      body: 'Your current key stops working immediately. Save the new one before closing this window.',
      confirm: 'Create new key',
    })
  )
    rotatedKey.value = (await auth.rotate.mutateAsync()).key
}
async function deleteAccount() {
  if (
    await confirm({
      title: 'Delete your account?',
      body: 'Your networks, saved logins, history and settings are deleted for good.',
      confirm: 'Delete account',
      danger: true,
      requireText: 'DELETE',
    })
  )
    await auth.deleteAccount()
}
const date = (iso?: string) =>
  iso
    ? new Date(iso).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      })
    : ''
</script>
<template>
  <div class="appearance-settings">
    <div class="account-summary">
      <AppIcon
        :name="username ? 'user' : 'key'"
        :size="20"
      />
      <span class="setting-copy"
        ><strong v-if="username">{{ username }}</strong
        ><strong v-else>Account key</strong
        ><small
          ><template v-if="username">Also your account on irc.ircord.com. </template>Created
          {{ date(auth.account?.createdAt) }}. Deleted after 90 days without a visit (currently
          {{ date(auth.account?.expiresAt) }}).</small
        ></span
      >
    </div>
    <section
      v-if="username"
      class="setting-group"
    >
      <h3>Password</h3>
      <form
        class="password-form"
        @submit.prevent="changePassword"
      >
        <input
          :value="username"
          autocomplete="username"
          hidden
        />
        <label
          class="field-label"
          for="current-password"
          >Current password</label
        >
        <input
          id="current-password"
          v-model="current"
          class="text-input"
          type="password"
          autocomplete="current-password"
          required
        />
        <label
          class="field-label"
          for="new-password"
          >New password</label
        >
        <input
          id="new-password"
          v-model="next"
          class="text-input"
          type="password"
          autocomplete="new-password"
          minlength="8"
          maxlength="128"
          required
          aria-describedby="new-password-hint"
        />
        <p
          id="new-password-hint"
          class="field-hint"
        >
          At least 8 characters. It changes on irc.ircord.com too.
        </p>
        <p
          v-if="passwordError"
          class="form-error"
          role="alert"
        >
          {{ passwordError }}
        </p>
        <p
          v-else-if="passwordSaved"
          class="field-hint"
          role="status"
        >
          Password changed.
        </p>
        <div class="form-actions">
          <button
            class="button primary small"
            :disabled="!current || next.length < 8 || auth.changePassword.isLoading"
          >
            Change password
          </button>
        </div>
      </form>
    </section>
    <section
      v-else
      class="setting-group"
    >
      <h3>Username and password</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Choose a username and password</strong
          ><small
            >Account keys are going away. Your username is also your account on
            irc.ircord.com.</small
          ></span
        >
        <button
          class="button primary small"
          @click="choosing = true"
        >
          Choose
        </button>
      </div>
      <div
        v-if="rotatedKey"
        class="setting-row stacked"
      >
        <span class="setting-copy"
          ><strong>Your new key</strong><small>Save it now. It isn’t shown again.</small></span
        >
        <div class="account-key">{{ rotatedKey }}</div>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Replace your key</strong
          ><small>If your key may have leaked. The old one stops working.</small></span
        >
        <button
          class="button secondary small"
          :disabled="auth.rotate.isLoading"
          @click="rotateKey"
        >
          Create new key
        </button>
      </div>
      <LazyCredentialsPrompt
        v-if="choosing"
        @close="choosing = false"
      />
    </section>
    <section class="setting-group">
      <h3>Session</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Sign out</strong
          ><small>{{
            username
              ? 'Sign back in with your username and password.'
              : 'You’ll need your key to sign back in.'
          }}</small></span
        >
        <button
          class="button secondary small"
          @click="auth.logout()"
        >
          Sign out
        </button>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Delete account</strong
          ><small>Removes your networks, logins, history and settings.</small></span
        >
        <button
          class="button small danger"
          @click="deleteAccount"
        >
          Delete account
        </button>
      </div>
    </section>
  </div>
</template>
