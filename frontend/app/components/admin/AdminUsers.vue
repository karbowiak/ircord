<script setup vapor lang="ts">
import { useQueryCache } from '@pinia/colada'
import type { components } from '../../../shared/generated/api'

type User = components['schemas']['AdminUser']

/**
 * Administration → Users: every IRCord account, owners first, searchable and paged. For now the
 * only action is server ownership; retired-name release (#166) and channel handover (#167) are
 * their own pages.
 */
const PAGE = 50
const query = ref('')
const users = ref<User[]>([])
const more = ref(false)
const loading = ref(false)
const error = ref<unknown>()
const notice = ref<{ key: string; values?: Record<string, string | number> } | null>(null)
const { t, n } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))
const noticeText = computed(() =>
  notice.value ? t(notice.value.key, notice.value.values ?? {}) : '',
)
const date = (value: string) => new Date(value).toLocaleDateString()
// The notice mounts with its text, so it is announced through the persistent announcer.
const { say } = useStatusAnnouncer()
watch(noticeText, (text) => say(text))
const busy = ref('')
const refused = useAdministrationRefusal()
const queryCache = useQueryCache()
const { ask } = usePrompt()
const { full } = useTimeFormat()
const auth = useAuthStore()
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined

async function load(append = false) {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = undefined
  try {
    const page = await apiClient().administrationUsers(
      {
        query: query.value.trim() || undefined,
        offset: append ? users.value.length : 0,
        limit: PAGE,
      },
      controller.signal,
    )
    users.value = append ? [...users.value, ...page.items] : page.items
    more.value = page.more
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    refused(e)
    error.value = e
  } finally {
    loading.value = false
  }
}
watch(query, () => {
  clearTimeout(timer)
  timer = setTimeout(() => load(), 250)
})
onMounted(() => load())
onBeforeUnmount(() => {
  clearTimeout(timer)
  controller?.abort()
})

function replace(user: User) {
  const index = users.value.findIndex((u) => u.id === user.id)
  if (index >= 0) users.value.splice(index, 1, user)
  else users.value.unshift(user)
  void queryCache.invalidateQueries({ key: ['administration', 'audit'] })
}
async function grant(user: User) {
  if (user.owner) {
    notice.value = { key: 'administration.alreadyOwner' }
    return
  }
  const reason = await ask({
    title: t('collectionsUi.admin.makeOwnerConfirm', { username: user.username }),
    body: t('collectionsUi.admin.makeOwnerHelp'),
    label: t('collectionsUi.admin.reasonOptional'),
    confirm: t('collectionsUi.admin.makeOwner'),
    allowEmpty: true,
  })
  if (reason === null) return
  await change(
    user,
    () => apiClient().grantOwner(user.id, reason || undefined),
    'collectionsUi.admin.userMadeOwner',
  )
}
async function revoke(user: User) {
  const self = user.id === auth.account?.id
  const reason = await ask({
    title: self
      ? t('collectionsUi.admin.stopBeingOwner')
      : t('collectionsUi.admin.removeOwnerConfirm', { username: user.username }),
    body: self
      ? t('collectionsUi.admin.loseOwnAccess')
      : t('collectionsUi.admin.loseUserAccess', { username: user.username }),
    label: t('collectionsUi.admin.reasonOptional'),
    confirm: t('collectionsUi.admin.removeOwner'),
    danger: true,
    allowEmpty: true,
  })
  if (reason === null) return
  await change(
    user,
    () => apiClient().revokeOwner(user.id, reason || undefined),
    'collectionsUi.admin.userNoLongerOwner',
  )
}
async function change(user: User, run: () => Promise<User>, done: string) {
  busy.value = user.id
  error.value = undefined
  notice.value = null
  try {
    replace(await run())
    notice.value = { key: done, values: { username: user.username } }
    if (user.id === auth.account?.id)
      void queryCache.invalidateQueries({ key: ['administration', 'access'] })
  } catch (e) {
    refused(e)
    error.value = e
  } finally {
    busy.value = ''
  }
}
const sources = computed<Record<string, string>>(() => ({
  environment: t('collectionsUi.admin.ownerSourceEnvironment'),
  operator: t('collectionsUi.admin.ownerSourceOperator'),
  owner: t('collectionsUi.admin.ownerSourceOwner'),
}))
</script>
<template>
  <div class="admin-page grid gap-7">
    <SettingGroup :title="t('collectionsUi.admin.addOwner')">
      <SettingsDescription>
        {{ t('collectionsUi.admin.addOwnerHelp') }}
      </SettingsDescription>
      <AdminAccountPicker
        :label="t('collectionsUi.admin.usernameOrNickname')"
        :action="t('collectionsUi.admin.makeOwner')"
        :busy="!!busy"
        @choose="grant"
      />
    </SettingGroup>
    <SettingGroup :title="t('collectionsUi.admin.accounts')">
      <UiInput
        v-model="query"
        class="admin-search w-full mb-2"
        type="search"
        :placeholder="t('collectionsUi.admin.filterByUsername')"
        :aria-label="t('collectionsUi.admin.filterAccounts')"
        maxlength="64"
      />
      <FieldHint v-if="notice">
        {{ noticeText }}
      </FieldHint>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ errorMessage(error) }}
      </UiAlert>
      <ul
        class="admin-users list-none m-0 p-0 grid gap-2 [&_li]:flex [&_li]:items-center [&_li]:gap-3 [&_li]:rounded-[var(--radius-large)] [&_li]:bg-card [&_li]:px-3.5 [&_li]:py-3 [&_li]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
      >
        <li
          v-for="user in users"
          :key="user.id"
        >
          <span
            aria-hidden="true"
            class="grid size-9 shrink-0 place-items-center rounded-full bg-selection text-[14px] font-[700] text-primary uppercase"
            >{{ user.username.slice(0, 1) }}</span
          >
          <div
            class="admin-user-copy flex-1 min-w-0 grid gap-0.5 [&_small]:text-muted-foreground [&_small]:text-[12px]"
          >
            <strong
              >{{ user.username
              }}<span
                v-if="user.owner"
                class="admin-badge ml-2 py-[1px] px-1.5 rounded-[4px] [background:color-mix(in_srgb,_var(--accent)_20%,_transparent)] text-primary text-[11px] font-[700] [vertical-align:1px]"
                :title="sources[user.ownerSource ?? 'owner']"
                >{{ t('collectionsUi.admin.ownerBadge') }}</span
              ><span
                v-if="user.id === auth.account?.id"
                class="admin-you text-muted-foreground font-[400]"
              >
                {{ t('collectionsUi.admin.you') }}</span
              ></strong
            >
            <small>
              <time
                :datetime="user.createdAt"
                :title="full(user.createdAt)"
                >{{ t('collectionsUi.admin.joined', { date: date(user.createdAt) }) }}</time
              >
              ·
              <time
                :datetime="user.lastSeenAt"
                :title="full(user.lastSeenAt)"
                >{{ t('collectionsUi.admin.lastSeen', { date: date(user.lastSeenAt) }) }}</time
              >
              ·
              {{ t('collectionsUi.admin.sessions', { count: n(user.sessions) }, user.sessions) }} ·
              {{ t('collectionsUi.admin.networks', { count: n(user.networks) }, user.networks) }}
            </small>
          </div>
          <UiButton
            v-if="!user.owner"
            variant="secondary"
            size="sm"
            :disabled="!!busy"
            :aria-label="t('collectionsUi.admin.makeUserOwner', { username: user.username })"
            @click="grant(user)"
          >
            {{ t('collectionsUi.admin.makeOwner') }}
          </UiButton>
          <UiButton
            v-else-if="user.ownerSource !== 'environment'"
            variant="secondary"
            size="sm"
            :disabled="!!busy"
            :aria-label="t('collectionsUi.admin.removeUserOwner', { username: user.username })"
            @click="revoke(user)"
          >
            {{ t('collectionsUi.admin.removeOwner') }}
          </UiButton>
          <small
            v-else
            class="admin-locked max-w-[14ch] text-faint text-[11px] text-right"
            >{{ sources.environment }}</small
          >
        </li>
      </ul>
      <SettingsDescription v-if="!loading && !users.length && !error">
        {{ t('collectionsUi.admin.noAccountsMatch', { query: query.trim() }) }}
      </SettingsDescription>
      <UiButton
        v-if="more"
        variant="secondary"
        size="sm"
        :disabled="loading"
        @click="load(true)"
      >
        {{ t('collectionsUi.admin.showMore') }}
      </UiButton>
    </SettingGroup>
  </div>
</template>
