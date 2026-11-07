<script setup lang="ts">
/** Settings → Account: the anonymous key account, its key, sign-out and deletion. */
const auth = useAuthStore()
const { confirm } = usePrompt()
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
        name="key"
        :size="20"
      />
      <span class="setting-copy"
        ><strong>Anonymous account</strong
        ><small
          >Created {{ date(auth.account?.createdAt) }}. Deleted after 90 days without a visit
          (currently {{ date(auth.account?.expiresAt) }}).</small
        ></span
      >
    </div>
    <section class="setting-group">
      <h3>Account key</h3>
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
    </section>
    <section class="setting-group">
      <h3>Session</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Sign out</strong><small>You’ll need your key to sign back in.</small></span
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
