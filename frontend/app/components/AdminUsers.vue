<script setup lang="ts">
import { useQueryCache } from '@pinia/colada'
import type { components } from '../../shared/generated/api'

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
const error = ref('')
const notice = ref('')
// The notice mounts with its text, so it is announced through the persistent announcer.
const { say } = useStatusAnnouncer()
watch(notice, (text) => say(text))
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
  error.value = ''
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
    error.value = errorText(e)
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
    notice.value = `${user.username} is already an owner.`
    return
  }
  const reason = await ask({
    title: `Make ${user.username} an owner?`,
    body: 'Owners can open Administration, see every account and add or remove other owners.',
    label: 'Reason (optional, kept in the log)',
    confirm: 'Make owner',
    allowEmpty: true,
  })
  if (reason === null) return
  await change(user, () => apiClient().grantOwner(user.id, reason || undefined), 'is now an owner')
}
async function revoke(user: User) {
  const self = user.id === auth.account?.id
  const reason = await ask({
    title: self ? 'Stop being an owner?' : `Remove ${user.username} as owner?`,
    body: self
      ? 'You lose access to Administration straight away.'
      : `${user.username} loses access to Administration straight away. Their account is otherwise unchanged.`,
    label: 'Reason (optional, kept in the log)',
    confirm: 'Remove owner',
    danger: true,
    allowEmpty: true,
  })
  if (reason === null) return
  await change(
    user,
    () => apiClient().revokeOwner(user.id, reason || undefined),
    'is no longer an owner',
  )
}
async function change(user: User, run: () => Promise<User>, done: string) {
  busy.value = user.id
  error.value = ''
  notice.value = ''
  try {
    replace(await run())
    notice.value = `${user.username} ${done}.`
    if (user.id === auth.account?.id)
      void queryCache.invalidateQueries({ key: ['administration', 'access'] })
  } catch (e) {
    refused(e)
    error.value = errorText(e)
  } finally {
    busy.value = ''
  }
}
const sources: Record<string, string> = {
  environment: 'Set in the server configuration',
  operator: 'Granted by the server operator',
  owner: 'Granted in Administration',
}
</script>
<template>
  <div class="admin-page grid gap-2">
    <SettingGroup title="Add an owner">
      <SettingsDescription>
        Search by username, or by the nickname someone uses on the IRC server.
      </SettingsDescription>
      <AdminAccountPicker
        label="Username or nickname"
        action="Make owner"
        :busy="!!busy"
        @choose="grant"
      />
    </SettingGroup>
    <SettingGroup title="Accounts">
      <UiInput
        v-model="query"
        class="admin-search w-full mb-2"
        type="search"
        placeholder="Filter by username"
        aria-label="Filter accounts by username"
        maxlength="64"
      />
      <FieldHint v-if="notice">
        {{ notice }}
      </FieldHint>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <ul
        class="admin-users list-none m-0 p-0 grid [&amp;_li]:flex [&amp;_li]:items-center [&amp;_li]:gap-3 [&amp;_li]:px-0 [&amp;_li]:py-2 [&amp;_li]:border-b [&amp;_li]:border-b-border [&amp;_li]:[border-bottom-style:solid]"
      >
        <li
          v-for="user in users"
          :key="user.id"
        >
          <div
            class="admin-user-copy flex-1 min-w-0 grid gap-0.5 [&amp;_small]:text-muted-foreground [&amp;_small]:text-[12px]"
          >
            <strong
              >{{ user.username
              }}<span
                v-if="user.owner"
                class="admin-badge ml-2 py-[1px] px-1.5 rounded-[4px] [background:color-mix(in_srgb,_var(--accent)_20%,_transparent)] text-primary text-[11px] font-[700] [vertical-align:1px]"
                :title="sources[user.ownerSource ?? 'owner']"
                >Owner</span
              ><span
                v-if="user.id === auth.account?.id"
                class="admin-you text-muted-foreground font-[400]"
              >
                (you)</span
              ></strong
            >
            <small>
              Joined
              <time
                :datetime="user.createdAt"
                :title="full(user.createdAt)"
                >{{ new Date(user.createdAt).toLocaleDateString() }}</time
              >
              · last seen
              <time
                :datetime="user.lastSeenAt"
                :title="full(user.lastSeenAt)"
                >{{ new Date(user.lastSeenAt).toLocaleDateString() }}</time
              >
              · {{ user.sessions }} {{ user.sessions === 1 ? 'session' : 'sessions' }} ·
              {{ user.networks }} {{ user.networks === 1 ? 'network' : 'networks' }}
            </small>
          </div>
          <UiButton
            v-if="!user.owner"
            variant="secondary"
            size="sm"
            :disabled="!!busy"
            :aria-label="`Make ${user.username} an owner`"
            @click="grant(user)"
          >
            Make owner
          </UiButton>
          <UiButton
            v-else-if="user.ownerSource !== 'environment'"
            variant="secondary"
            size="sm"
            :disabled="!!busy"
            :aria-label="`Remove ${user.username} as owner`"
            @click="revoke(user)"
          >
            Remove owner
          </UiButton>
          <small
            v-else
            class="admin-locked max-w-[14ch] text-faint text-[11px] text-right"
            >{{ sources.environment }}</small
          >
        </li>
      </ul>
      <SettingsDescription v-if="!loading && !users.length && !error">
        No accounts match “{{ query.trim() }}”.
      </SettingsDescription>
      <UiButton
        v-if="more"
        variant="secondary"
        size="sm"
        :disabled="loading"
        @click="load(true)"
      >
        Show more
      </UiButton>
    </SettingGroup>
  </div>
</template>
