import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type { Credentials, SignedInAccount } from '../../shared/types/auth'

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
 * Username and password accounts (#64); the username is also the irc.ircord.com account. Older
 * key accounts still sign in with their key and are asked to choose a username and password.
 * Either is exchanged for an HttpOnly session cookie and never kept by the browser.
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
        const account = (await apiClient().session()).account as SignedInAccount | null
        if (account) writeLocal(LAST, account)
        else forgetDevice()
        return account
      } catch (error) {
        // The server still checks the cookie on every request; this only decides what to show.
        const last = readLocal<SignedInAccount | null>(LAST, (v) =>
          v && typeof v === 'object' && 'id' in v ? (v as SignedInAccount) : null,
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
  function enter(value: SignedInAccount) {
    queryCache.setQueryData(key, value)
    // Account-owned data loaded while signed out (or for another account) is stale now.
    void queryCache.invalidateQueries({ key: ['appearance'] })
    void queryCache.invalidateQueries({ key: ['library'] })
    // A fresh page also clears disposed transports and account-owned query caches.
    location.reload()
  }

  const signup = useMutation({
    mutation: (body: Credentials & { registrationKey?: string }) => accountClient.signup(body),
    onSuccess: (result) => enter(result.account),
  })
  const login = useMutation({
    mutation: (body: Credentials) => accountClient.login(body),
    onSuccess: (result) => enter(result.account),
  })
  /** Accounts from before #64, until keys are retired. */
  const keyLogin = useMutation({
    mutation: (value: string) => apiClient().login(value),
    onSuccess: (result) => enter(result.account),
  })
  /** A key account choosing its username and password; it stays signed in. */
  const setCredentials = useMutation({
    mutation: (body: Credentials) => accountClient.setCredentials(body),
    onSuccess: (result) => {
      queryCache.setQueryData(key, result.account)
      writeLocal(LAST, result.account)
    },
  })
  const changePassword = useMutation({
    mutation: (body: { current: string; password: string }) => accountClient.changePassword(body),
  })
  const rotate = useMutation({
    mutation: () => apiClient().rotate(),
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
    keyLogin,
    setCredentials,
    changePassword,
    rotate,
    logout,
    deleteAccount,
    expire,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useAuthStore, import.meta.hot))
