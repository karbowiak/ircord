import type { InjectionKey } from 'vue'
import type { NetworkInput } from '../../../shared/types/chat'
import { maxIconBytes } from '../../../shared/preferences'
import { networkPresets, normalizeNetworkHost, type NetworkPreset } from '../../../shared/network-presets'
import { nicknameError, nicknameLimit } from '../../../shared/utils/nickname'
import { prepareSquareImage } from '../../utils/prepare-square-image'

type Method = NetworkInput['auth']['method']

export interface NetworkFormProps {
  networkId: string
  prefill: Partial<NetworkInput>
  firstConnection: boolean
}

/**
 * The add/edit network form's state: fields, server verification, nick validity, the icon and
 * saving. NetworkDialog creates it and its sections share it, so each fieldset can be its own
 * component without passing the form around. `done` runs after saving or removing.
 */
export function createNetworkForm(props: NetworkFormProps, done: () => void) {
  const { t } = useI18n()
  const chat = useChatStore()
  const auth = useAuthStore()
  const preferences = usePreferencesStore()
  const { confirm } = usePrompt()
  const existing = computed(() => chat.data?.networks.find((n) => n.id === props.networkId))
  const editing = computed(() => !!existing.value)
  const fallbackPresets = networkPresets(useRuntimeConfig().public.networkPresets)
  const presets = ref<NetworkPreset[]>([])

  const n = existing.value
  /** New networks can start from a prefill (an irc:// link). */
  const p: Partial<NetworkInput> = { ...props.prefill }
  const initialMethod = (n?.auth.method ?? p.auth?.method ?? 'none') as Method
  const form = reactive({
    name: n?.name ?? p.name ?? '',
    host: n?.host ?? p.host ?? '',
    port: n?.port ?? p.port ?? 6697,
    tls: n?.tls ?? p.tls ?? true,
    nick: n?.nick ?? p.nick ?? (props.firstConnection ? '' : (chat.data?.user.nick ?? '')),
    altNick: n?.altNick ?? '',
    username: n?.username ?? '',
    realname: n?.realname ?? p.realname ?? '',
    method: (initialMethod === 'sasl-external' ? 'none' : initialMethod) as Method,
    account: n?.auth.method === 'sasl-plain' ? n.auth.account : '',
    password: '',
    autoJoin: (n?.autoJoin ?? p.autoJoin ?? []).join('\n'),
    perform: (n?.perform ?? []).join('\n'),
  })
  const initialPresetFields = {
    name: form.name, host: form.host, port: form.port, tls: form.tls, method: form.method,
    autoJoin: form.autoJoin, password: form.password, account: form.account,
  }
  /** The stored password stays unless the user types a new one or clears it. */
  const providedPassword = computed(
    () =>
      !editing.value &&
      presets.value.some(
        (preset) =>
          preset.passwordProvided &&
          preset.host === normalizeNetworkHost(form.host) &&
          preset.port === Number(form.port) &&
          preset.tls === form.tls &&
          preset.authMethod === form.method,
      ),
  )
  const passwordSet = computed(() => {
    const auth = existing.value?.auth as { passwordSet?: boolean } | undefined
    return (
      providedPassword.value || (!!auth?.passwordSet && existing.value?.auth.method === form.method)
    )
  })
  const clearPassword = ref(false)
  const validEndpoint = (host: string, port: number) =>
    !!normalizeNetworkHost(host) &&
    Number.isInteger(port) &&
    port >= 1 &&
    port <= 65535
  const advanced = ref(
    !form.name.trim() ||
      !validEndpoint(form.host, Number(form.port)) ||
      (form.method !== 'none' && !passwordSet.value),
  )
  const setupNick = ref<HTMLInputElement | null>(null)
  const standardNick = ref<HTMLInputElement | null>(null)
  const alternateNick = ref<HTMLInputElement | null>(null)
  const hostInput = ref<HTMLInputElement | null>(null)
  const nickMax = computed(() => nicknameLimit(existing.value?.isupport))
  const nickProblem = computed(() => nicknameError(form.nick, nickMax.value, t))
  const hostProblem = computed(() =>
    form.host && !normalizeNetworkHost(form.host) ? t('networkForm.invalidHostname') : '',
  )
  const alternateProblem = computed(() =>
    form.altNick ? nicknameError(form.altNick, nickMax.value, t) : '',
  )
  watchEffect(() => {
    setupNick.value?.setCustomValidity(nickProblem.value)
    standardNick.value?.setCustomValidity(nickProblem.value)
    alternateNick.value?.setCustomValidity(alternateProblem.value)
    hostInput.value?.setCustomValidity(hostProblem.value)
  })
  onMounted(() => {
    if (props.firstConnection) void nextTick(() => setupNick.value?.focus())
  })
  function revealInvalidField(event: Event) {
    if (props.firstConnection && !advanced.value && event.target !== setupNick.value) {
      event.preventDefault()
      advanced.value = true
      const input = event.target as HTMLInputElement
      void nextTick(() => {
        input.focus()
        input.reportValidity()
      })
    }
  }
  const error = ref('')
  const icon = ref<string | null | undefined>(undefined)
  const iconPreview = computed(() =>
    icon.value === undefined ? preferences.preferences.networkIcons[props.networkId] : icon.value,
  )
  const iconInput = ref<HTMLInputElement | null>(null)
  const saving = computed(() => chat.addNetwork.isLoading || chat.updateNetwork.isLoading)
  const verification = ref({ state: 'idle', detail: '' })
  const checkAttempt = ref(0)
  const needsVerification = computed(
    () => !n || normalizeNetworkHost(form.host) !== normalizeNetworkHost(n.host) || Number(form.port) !== n.port || form.tls !== n.tls,
  )
  watch(
    () => [normalizeNetworkHost(form.host) ?? '', Number(form.port), form.tls, checkAttempt.value] as const,
    ([host, port, tls], _previous, onCleanup) => {
      const controller = new AbortController()
      onCleanup(() => controller.abort())
      if (!needsVerification.value) {
        verification.value = { state: 'verified', detail: '' }
        return
      }
      if (!host || !Number.isInteger(port) || port < 1 || port > 65535) {
        if (props.firstConnection) advanced.value = true
        verification.value = { state: 'idle', detail: '' }
        return
      }
      verification.value = { state: 'checking', detail: '' }
      const timer = setTimeout(async () => {
        try {
          const result = await apiClient().checkNetwork(
            { host, port, tls },
            AbortSignal.any([controller.signal, AbortSignal.timeout(8000)]),
          )
          if (!controller.signal.aborted) {
            if (!result.reachable && props.firstConnection) advanced.value = true
            verification.value = {
              state: result.reachable ? 'verified' : 'error',
              detail: result.detail,
            }
          }
        } catch (cause) {
          if (!controller.signal.aborted) {
            if (props.firstConnection) advanced.value = true
            verification.value = { state: 'error', detail: errorText(cause, t) }
          }
        }
      }, 300)
      controller.signal.addEventListener('abort', () => clearTimeout(timer))
    },
    { immediate: true },
  )
  const methods = computed<{ value: Method; label: string; note: string }[]>(() => [
    {
      value: 'none',
      label: t('networkForm.methods.none'),
      note: t('networkForm.methods.noneHelp'),
    },
    {
      value: 'sasl-plain',
      label: 'SASL',
      note: t('networkForm.methods.saslHelp'),
    },
    { value: 'nickserv', label: 'NickServ', note: t('networkForm.methods.nickservHelp') },
    {
      value: 'server-password',
      label: t('networkForm.methods.serverPassword'),
      note: t('networkForm.methods.serverPasswordHelp'),
    },
  ])
  function selectPreset(preset: NetworkPreset) {
    form.name = preset.name
    form.host = preset.host
    form.port = preset.port
    form.tls = preset.tls
    form.method = preset.authMethod
    form.autoJoin = preset.autoJoin.join('\n')
    form.password = ''
    form.account = ''
    clearPassword.value = false
  }
  // Never replace a form that the user has touched while the request is in flight.
  let edited = false
  let applyingDefault = false
  let autoApplied = false
  let presetAccount: string | undefined
  watch(
    form,
    () => {
      if (!applyingDefault) edited = true
    },
    { deep: true, flush: 'sync' },
  )
  onMounted(() => {
    watch(
      () => (auth.status === 'signed-in' ? auth.account?.id : undefined),
      (accountId, _previous, onCleanup) => {
        if (presetAccount !== accountId && autoApplied && !edited) {
          applyingDefault = true
          Object.assign(form, initialPresetFields)
          applyingDefault = false
          advanced.value = true
          autoApplied = false
        }
        presetAccount = accountId
        presets.value = []
        if (!accountId) return
        const controller = new AbortController()
        onCleanup(() => controller.abort())
        const apply = (items: NetworkPreset[]) => {
          if (controller.signal.aborted || auth.status !== 'signed-in' || auth.account?.id !== accountId)
            return
          presets.value = items
          if (
            props.firstConnection &&
            !props.networkId &&
            !Object.keys(props.prefill).length &&
            !edited &&
            items[0]
          ) {
            applyingDefault = true
            selectPreset(items[0])
            applyingDefault = false
            autoApplied = true
            advanced.value =
              !form.name.trim() ||
              !validEndpoint(form.host, Number(form.port)) ||
              (form.method !== 'none' && !passwordSet.value)
          }
        }
        void apiClient()
          .networkPresets(controller.signal)
          .then((result) => apply(result.configured ? networkPresets(result.items, true) : fallbackPresets))
          .catch(() => apply(fallbackPresets))
      },
      { immediate: true, flush: 'sync' },
    )
  })
  const lines = (value: string) =>
    value
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)

  async function chooseIcon(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0]
    ;(event.target as HTMLInputElement).value = ''
    if (!file) return
    try {
      const url = await prepareSquareImage(file, 96)
      if (url.length > maxIconBytes) throw new Error(t('networkForm.imageTooDetailed'))
      icon.value = url
    } catch (e) {
      error.value = e instanceof Error ? e.message : t('networkForm.imageOpenFailed')
    }
  }

  async function submit() {
    if (needsVerification.value && verification.value.state !== 'verified') {
      if (props.firstConnection && !advanced.value) {
        advanced.value = true
        void nextTick(() => {
          document.getElementById('network-host')?.focus()
        })
      }
      return
    }
    error.value = ''
    const password = form.password ? form.password : clearPassword.value ? '' : undefined
    const auth: NetworkInput['auth'] =
      form.method === 'sasl-plain'
        ? { method: 'sasl-plain', account: form.account, password }
        : form.method === 'nickserv' || form.method === 'server-password'
          ? { method: form.method, password }
          : { method: form.method }
    const input: NetworkInput = {
      name: form.name,
      host: normalizeNetworkHost(form.host) ?? form.host,
      port: Number(form.port),
      tls: form.tls,
      nick: form.nick,
      altNick: form.altNick || undefined,
      username: form.username || undefined,
      realname: form.realname || undefined,
      auth,
      autoJoin: lines(form.autoJoin).map((entry) => (/^[#&]/.test(entry) ? entry : `#${entry}`)),
      perform: lines(form.perform),
    }
    try {
      const id = editing.value
        ? (await chat.updateNetwork.mutateAsync({ id: props.networkId, input })).id
        : (await chat.addNetwork.mutateAsync(input)).network.id
      if (icon.value !== undefined) preferences.setIcon(id, icon.value)
      done()
    } catch (e) {
      error.value = errorText(e, t)
    }
  }
  async function remove() {
    const network = existing.value
    if (!network) return
    if (
      await confirm({
        title: t('networkForm.removeTitle', { name: network.name }),
        body: t('networkForm.removeDescription'),
        confirm: t('common.remove'),
        danger: true,
        requireText: network.name,
      })
    ) {
      try {
        await chat.removeNetwork.mutateAsync(network)
        preferences.setIcon(network.id, null)
        done()
      } catch (cause) {
        error.value = errorText(cause, t)
      }
    }
  }
  const initialForm = JSON.stringify(form)
  /** True once anything differs from what the dialog opened with. */
  const dirty = computed(
    () => JSON.stringify(form) !== initialForm || icon.value !== undefined || clearPassword.value,
  )
  const state = {
    dirty,
    firstConnection: props.firstConnection,
    existing,
    editing,
    presets,
    form,
    providedPassword,
    passwordSet,
    clearPassword,
    advanced,
    setupNick,
    standardNick,
    alternateNick,
    hostInput,
    nickMax,
    nickProblem,
    hostProblem,
    alternateProblem,
    revealInvalidField,
    error,
    icon,
    iconPreview,
    iconInput,
    saving,
    verification,
    checkAttempt,
    needsVerification,
    methods,
    selectPreset,
    chooseIcon,
    submit,
    remove,
  }
  provide(networkFormKey, state)
  return state
}

export type NetworkForm = ReturnType<typeof createNetworkForm>
const networkFormKey: InjectionKey<NetworkForm> = Symbol('network-form')

/** The form a NetworkDialog section belongs to. */
export function useNetworkForm() {
  const state = inject(networkFormKey)
  if (!state) throw new Error('useNetworkForm needs a NetworkDialog around it')
  return state
}
