import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type { Account } from '../../shared/types/chat'

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
 * Anonymous key accounts. The key is exchanged for an HttpOnly session cookie and is never kept
 * by the browser; after registering it is held in memory only until the user confirms saving it.
 */
export const useAuthStore = defineStore('auth', () => {
  const queryCache = useQueryCache()
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
  /** A freshly created key waiting for the user to confirm they saved it. */
  const newKey = ref('')
  let pendingAccount: Account | null = null

  function enter(value: Account) {
    queryCache.setQueryData(key, value)
    // Account-owned data loaded while signed out (or for another account) is stale now.
    void queryCache.invalidateQueries({ key: ['appearance'] })
    void queryCache.invalidateQueries({ key: ['library'] })
    // A fresh page also clears disposed transports and account-owned query caches.
    location.reload()
  }

  const register = useMutation({
    mutation: () => apiClient().register(),
    onSuccess: (result) => {
      newKey.value = result.key
      pendingAccount = result.account
    },
  })
  function confirmKey() {
    newKey.value = ''
    if (pendingAccount) enter(pendingAccount)
    pendingAccount = null
  }
  const login = useMutation({
    mutation: (value: string) =>
      apiClient().login(value),
    onSuccess: (result) => enter(result.account),
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
    session,
    account,
    status,
    newKey,
    register,
    confirmKey,
    login,
    rotate,
    logout,
    deleteAccount,
    expire,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useAuthStore, import.meta.hot))
