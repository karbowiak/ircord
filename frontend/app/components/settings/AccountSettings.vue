<script setup vapor lang="ts">
/** Settings → Account: your username and password, logging out and deletion. */
const auth = useAuthStore()
const { ask } = usePrompt()
const { t, d } = useI18n()
const $t = t
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
    passwordError.value = errorText(e, (key) => t(key))
  }
}

const deleteError = ref('')
async function deleteAccount() {
  deleteError.value = ''
  const password = await ask({
    title: t('account.deleteConfirm.title'),
    body: t('account.deleteConfirm.body'),
    label: t('account.currentPassword'),
    confirm: t('account.deleteAccount'),
    danger: true,
    password: true,
  })
  if (!password) return
  try {
    await auth.deleteAccount(password)
  } catch (e) {
    deleteError.value = errorText(e, (key) => t(key))
  }
}
const date = (iso?: string) =>
  iso
    ? d(new Date(iso), {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      })
    : ''
</script>
<template>
  <div class="appearance-settings grid gap-6">
    <div class="account-summary flex items-center gap-3 p-3.5 rounded-md bg-card text-primary">
      <AppIcon
        name="user"
        :size="20"
      />
      <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
        ><strong>{{ username }}</strong
        ><small>{{
          $t('account.summary', {
            created: date(auth.account?.createdAt),
            expires: date(auth.account?.expiresAt),
          })
        }}</small></span
      >
    </div>
    <SettingGroup
      :title="$t('account.password')"
      data-setting="password"
    >
      <form
        class="password-form grid gap-2 max-w-105"
        @submit.prevent="changePassword"
      >
        <input
          :value="username"
          autocomplete="username"
          hidden
        />
        <AppInput
          id="current-password"
          v-model="current"
          :label="$t('account.currentPassword')"
          type="password"
          autocomplete="current-password"
          required
        />
        <AppInput
          id="new-password"
          v-model="next"
          :label="$t('account.newPassword')"
          type="password"
          autocomplete="new-password"
          minlength="8"
          maxlength="128"
          required
          :help="$t('account.newPasswordHelp')"
          :error="passwordError || undefined"
          :status="passwordSaved ? $t('account.passwordChanged') : undefined"
        />
        <div class="form-actions flex justify-end items-center gap-2 mt-5">
          <UiButton
            size="sm"
            :disabled="!current || next.length < 8 || auth.changePassword.isLoading"
          >
            {{ $t('account.changePassword') }}
          </UiButton>
        </div>
      </form>
    </SettingGroup>
    <SettingGroup :title="$t('account.session')">
      <SettingRow
        data-setting="log-out"
        :label="$t('common.logOut')"
        :description="$t('account.logOutDescription')"
      >
        <UiButton
          variant="secondary"
          size="sm"
          @click="auth.confirmLogout()"
        >
          {{ $t('common.logOut') }}
        </UiButton>
      </SettingRow>
      <SettingRow
        data-setting="delete-account"
        :label="$t('account.deleteAccount')"
        :description="$t('account.deleteDescription')"
      >
        <UiButton
          variant="destructive"
          size="sm"
          @click="deleteAccount"
        >
          {{ $t('account.deleteAccount') }}
        </UiButton>
      </SettingRow>
      <UiAlert
        v-if="deleteError"
        as="p"
        variant="destructive"
        presentation="inline"
        role="alert"
        >{{ deleteError }}</UiAlert
      >
    </SettingGroup>
  </div>
</template>
