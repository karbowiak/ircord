import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type { Account } from '../../shared/types/chat'
import type { components } from '../../shared/generated/api'
import { readAloudStorage, speechKeyStorage } from '../../shared/speech-service'
import { offlineArchive } from '../utils/offline-archive'
import { clearLinkMediaCache } from '../utils/link-media-cache'

type Schema = components['schemas']
/** A default avatar's URL or an uploaded image, chosen while signing up. */
export type SignupAvatar = Pick<Schema['ProfileUpdate'], 'avatarUrl' | 'avatarDataUrl'>

const key = ['auth']
/** The last signed-in account on this device, so an unreachable server opens the offline copy. */
const LAST = 'ircord.lastAccount'
const unreachable = (error: unknown) => {
  const status = (error as { statusCode?: number } | null)?.statusCode
  return !status || status >= 500
}
/** Forgets this device's copy of the account (sign out, deletion, expiry). */
function forgetDevice(id?: string) {
  clearLinkMediaCache()
  try {
    if (id) localStorage.removeItem(`ircord.snapshot.${id}`)
    localStorage.removeItem(LAST)
    // Read-aloud's speech service key and switch belong to this device's signed-in account.
    localStorage.removeItem(speechKeyStorage)
    localStorage.removeItem(readAloudStorage)
  } catch {
    // Storage unavailable: nothing was kept.
  }
}

/**
 * Username and password accounts (#64); the username is also the IRC account on the associated
 * server. Signing in sets an HttpOnly session cookie; the password is never kept by the browser.
 */
export const useAuthStore = defineStore('auth', () => {
  const app = useNuxtApp()
  const queryCache = useQueryCache()
  const registration = useQuery({
    key: ['auth-registration'],
    query: () => apiClient().registration(),
    staleTime: 30_000,
  })
  const session = useQuery({
    key,
    query: async () => {
      try {
        const { account } = await apiClient().session()
        if (account) writeLocal(LAST, account)
        else {
          const last = readLocal<Account | null>(LAST, (v) =>
            v && typeof v === 'object' && 'id' in v ? (v as Account) : null,
          )
          if (last) void offlineArchive.clear(last.id)
          forgetDevice(last?.id)
          clearLinkMediaQueries()
        }
        return account
      } catch (error) {
        // The server still checks the cookie on every request; this only decides what to show.
        const last = readLocal<Account | null>(LAST, (v) =>
          v && typeof v === 'object' && 'id' in v ? (v as Account) : null,
        )
        if (last && unreachable(error)) return last
        if (!unreachable(error)) {
          if (last) void offlineArchive.clear(last.id)
          forgetDevice(last?.id)
          clearLinkMediaQueries()
        }
        throw error
      }
    },
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  })
  const account = computed(() => session.data.value ?? null)
  const status = computed(() =>
    session.data.value === undefined
      ? session.error.value
        ? 'error'
        : 'checking'
      : account.value
        ? 'signed-in'
        : 'signed-out',
  )
  function enter(value: Account) {
    clearLinkMediaCache()
    clearLinkMediaQueries()
    queryCache.setQueryData(key, value)
    // Account-owned data loaded while signed out (or for another account) is stale now.
    void queryCache.invalidateQueries({ key: ['appearance'] })
    void queryCache.invalidateQueries({ key: ['library'] })
    // A fresh page also clears disposed transports and account-owned query caches.
    location.reload()
  }

  const signup = useMutation({
    mutation: async ({
      avatar,
      ...body
    }: Schema['SignupCredentials'] & { avatar?: SignupAvatar }) => {
      const result = await apiClient().signup(body)
      // The picture chosen at sign-up goes through the normal profile save. The account exists
      // by now, so a picture that can't be saved doesn't fail sign-up; it can be set in settings.
      if (avatar)
        await apiClient()
          .profile({ bio: '', ...avatar })
          .catch(() => undefined)
      return result
    },
    onSuccess: (result) => enter(result.account),
  })
  const login = useMutation({
    mutation: (body: Schema['Credentials']) => apiClient().login(body),
    onSuccess: (result) => enter(result.account),
  })
  const changePassword = useMutation({
    mutation: (body: Schema['PasswordChange']) => apiClient().changePassword(body),
  })
  function clearLinkMediaQueries() {
    queryCache.cancelQueries({ key: ['link-media'] })
    queryCache.setQueriesData({ key: ['link-media'] }, () => undefined)
  }
  /** This device is signed out: forget its copy of the account and start fresh. */
  async function signedOut() {
    const cleanup = account.value ? offlineArchive.clear(account.value.id) : undefined
    forgetDevice(account.value?.id)
    clearLinkMediaQueries()
    queryCache.setQueryData(key, null)
    await cleanup
    location.reload()
  }
  // Signing out or deleting the account reloads, so no store keeps the old account's state.
  async function logout() {
    try {
      await apiClient().logout()
    } catch (error) {
      useChatStore().notify(errorText(error, (key) => app.$i18n.t(key)))
      return
    }
    await signedOut()
  }
  /** Asks "Log out of IRCord?" first; every sign-out control goes through here. */
  async function confirmLogout() {
    if (
      await usePrompt().confirm({
        title: app.$i18n.t('workspaceUi.shell.logoutTitle'),
        body: app.$i18n.t('workspaceUi.shell.logoutBody'),
        confirm: app.$i18n.t('common.logOut'),
        danger: true,
      })
    )
      await logout()
  }
  async function deleteAccount() {
    await apiClient().deleteAccount()
    await signedOut()
  }
  /** The server stopped recognising the session (expired or deleted elsewhere). */
  function expire() {
    if (account.value) void offlineArchive.clear(account.value.id)
    if (status.value === 'signed-in') useChatStore().dispose()
    forgetDevice(account.value?.id)
    clearLinkMediaQueries()
    queryCache.setQueryData(key, null)
  }

  useEventListener(window, 'ircord:session-expired', expire)

  return {
    registration,
    session,
    account,
    status,
    signup,
    login,
    changePassword,
    logout,
    signedOut,
    confirmLogout,
    deleteAccount,
    expire,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useAuthStore, import.meta.hot))
