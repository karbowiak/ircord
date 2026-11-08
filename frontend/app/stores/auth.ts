import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type { Account } from '../../shared/types/chat'
import type { components } from '../../shared/generated/api'

type Schema = components['schemas']

const key = ['auth']
/** The last signed-in account on this device, so an unreachable server opens the offline copy. */
const LAST = 'ircord.lastAccount'
const unreachable = (error: unknown) => {
  const status = (error as { statusCode?: number } | null)?.statusCode
  return !status || status >= 500
}
/** Forgets this device's copy of the account (sign out, deletion, expiry). */
function forgetDevice(id?: string) {
  try {
    if (id) localStorage.removeItem(`ircord.snapshot.${id}`)
    localStorage.removeItem(LAST)
  } catch {
    // Storage unavailable: nothing was kept.
  }
}

/**
 * Username and password accounts (#64); the username is also the IRC account on the associated
 * server. Signing in sets an HttpOnly session cookie; the password is never kept by the browser.
 */
export const useAuthStore = defineStore('auth', () => {
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
        else forgetDevice()
        return account
      } catch (error) {
        // The server still checks the cookie on every request; this only decides what to show.
        const last = readLocal<Account | null>(LAST, (v) =>
          v && typeof v === 'object' && 'id' in v ? (v as Account) : null,
        )
        if (last && unreachable(error)) return last
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
    queryCache.setQueryData(key, value)
    // Account-owned data loaded while signed out (or for another account) is stale now.
    void queryCache.invalidateQueries({ key: ['appearance'] })
    void queryCache.invalidateQueries({ key: ['library'] })
    // A fresh page also clears disposed transports and account-owned query caches.
    location.reload()
  }

  const signup = useMutation({
    mutation: (body: Schema['SignupCredentials']) => apiClient().signup(body),
    onSuccess: (result) => enter(result.account),
  })
  const login = useMutation({
    mutation: (body: Schema['Credentials']) => apiClient().login(body),
    onSuccess: (result) => enter(result.account),
  })
  const changePassword = useMutation({
    mutation: (body: Schema['PasswordChange']) => apiClient().changePassword(body),
  })
  // Signing out or deleting the account reloads, so no store keeps the old account's state.
  async function logout() {
    try {
      await apiClient().logout()
    } catch (error) {
      useChatStore().notify(errorText(error))
      return
    }
    forgetDevice(account.value?.id)
    location.reload()
  }
  /** Asks "Log out of IRCord?" first; every sign-out control goes through here. */
  async function confirmLogout() {
    if (
      await usePrompt().confirm({
        title: 'Log out of IRCord?',
        body: 'You can sign back in with your username and password.',
        confirm: 'Log out',
        danger: true,
      })
    )
      await logout()
  }
  async function deleteAccount() {
    await apiClient().deleteAccount()
    forgetDevice(account.value?.id)
    location.reload()
  }
  /** The server stopped recognising the session (expired or deleted elsewhere). */
  function expire() {
    if (status.value === 'signed-in') useChatStore().dispose()
    forgetDevice(account.value?.id)
    queryCache.setQueryData(key, null)
  }

  window.addEventListener('ircord:session-expired', expire)
  onScopeDispose(() => window.removeEventListener('ircord:session-expired', expire))

  return {
    registration,
    session,
    account,
    status,
    signup,
    login,
    changePassword,
    logout,
    confirmLogout,
    deleteAccount,
    expire,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useAuthStore, import.meta.hot))
