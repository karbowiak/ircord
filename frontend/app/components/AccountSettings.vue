<script setup lang="ts">
/** Settings → Account: your username and password, signed-in devices, logging out and deletion. */
const auth = useAuthStore()
const { confirm } = usePrompt()
const username = computed(() => auth.account?.username)

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
        name="user"
        :size="20"
      />
      <span class="setting-copy"
        ><strong>{{ username }}</strong
        ><small
          >Also your account on irc.ircord.com. Created {{ date(auth.account?.createdAt) }}. Deleted
          after 90 days without a visit (currently {{ date(auth.account?.expiresAt) }}).</small
        ></span
      >
    </div>
    <SettingGroup
      title="Password"
      data-setting="password"
    >
      <form
        class="password-form"
        @submit.prevent="changePassword"
      >
        <input
          :value="username"
          autocomplete="username"
          hidden
        />
        <AppInput
          v-model="current"
          label="Current password"
          id="current-password"
          type="password"
          autocomplete="current-password"
          required
        />
        <AppInput
          v-model="next"
          label="New password"
          id="new-password"
          type="password"
          autocomplete="new-password"
          minlength="8"
          maxlength="128"
          required
          help="At least 8 characters, no spaces. It changes on irc.ircord.com too."
          :error="passwordError || undefined"
          :status="passwordSaved ? 'Password changed.' : undefined"
        />
        <div class="form-actions">
          <button
            class="button primary small"
            :disabled="!current || next.length < 8 || auth.changePassword.isLoading"
          >
            Change password
          </button>
        </div>
      </form>
    </SettingGroup>
    <DeviceSessions />
    <SettingGroup title="Session">
      <SettingRow
        data-setting="log-out"
        label="Log out"
        description="On this device. Sign back in with your username and password."
      >
        <button
          class="button secondary small"
          @click="auth.confirmLogout()"
        >
          Log out
        </button>
      </SettingRow>
      <SettingRow
        data-setting="delete-account"
        label="Delete account"
        description="Removes your networks, logins, history and settings."
      >
        <button
          class="button small danger"
          @click="deleteAccount"
        >
          Delete account
        </button>
      </SettingRow>
    </SettingGroup>
  </div>
</template>
