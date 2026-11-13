import type { Ref } from 'vue'
import type { components } from '../../../shared/generated/api'

type UsernameCheck = components['schemas']['UsernameCheck']

/**
 * Checks a username against IRCord and irc.ircord.com while it's typed (debounced). Only the
 * latest answer counts; a failed lookup just drops the hint, since saving checks again.
 * Servers that require a sign-up key answer only with it, so `registrationKey` returns the key
 * when one is required (an empty key skips the check) and `undefined` otherwise.
 */
export function useUsernameCheck(
  username: Ref<string>,
  active: () => boolean = () => true,
  registrationKey: () => string | undefined = () => undefined,
) {
  const check = ref<UsernameCheck | null>(null)
  const checking = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let latest = 0
  // Also rechecks when it becomes active with a name already typed.
  watch([username, active, registrationKey], ([value]) => {
    clearTimeout(timer)
    const ticket = ++latest
    check.value = null
    checking.value = false
    const key = registrationKey()?.trim()
    if (!active() || !value.trim() || key === '') return
    checking.value = true
    timer = setTimeout(async () => {
      try {
        const result = await apiClient().checkUsername(value.trim(), key)
        if (ticket === latest) check.value = result
      } catch {
        if (ticket === latest) check.value = null
      } finally {
        if (ticket === latest) checking.value = false
      }
    }, 350)
  })
  onScopeDispose(() => clearTimeout(timer))
  return { check, checking }
}
