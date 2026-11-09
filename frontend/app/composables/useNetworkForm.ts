import type { InjectionKey } from 'vue'
import type { NetworkInput } from '../../shared/types/chat'
import { maxIconBytes } from '../../shared/preferences'
import { networkPresets, type NetworkPreset } from '../../shared/network-presets'
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'

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
  const preferences = usePreferencesStore()
  const { confirm } = usePrompt()
  const existing = computed(() => chat.data?.networks.find((n) => n.id === props.networkId))
  const editing = computed(() => !!existing.value)
  const presets = networkPresets(useRuntimeConfig().public.networkPresets)

  const n = existing.value
  /** New networks can start from a prefill (an irc:// link). */
  const firstPreset = props.firstConnection ? presets[0] : undefined
  const p: Partial<NetworkInput> = {
    ...(firstPreset
      ? {
          name: firstPreset.name,
          host: firstPreset.host,
          port: firstPreset.port,
          tls: firstPreset.tls,
          auth: { method: firstPreset.authMethod },
          autoJoin: firstPreset.autoJoin,
        }
      : {}),
    ...props.prefill,
  }
  const form = reactive({
    name: n?.name ?? p.name ?? '',
    host: n?.host ?? p.host ?? '',
    port: n?.port ?? p.port ?? 6697,
    tls: n?.tls ?? p.tls ?? true,
    nick: n?.nick ?? p.nick ?? (props.firstConnection ? '' : (chat.data?.user.nick ?? '')),
    altNick: n?.altNick ?? '',
    username: n?.username ?? '',
    realname: n?.realname ?? p.realname ?? '',
    method: (n?.auth.method ?? p.auth?.method ?? 'none') as Method,
    account: n?.auth.method === 'sasl-plain' ? n.auth.account : '',
    password: '',
    autoJoin: (n?.autoJoin ?? p.autoJoin ?? []).join('\n'),
    perform: (n?.perform ?? []).join('\n'),
  })
  /** The stored password stays unless the user types a new one or clears it. */
  const providedPassword = computed(
    () =>
      !editing.value &&
      presets.some(
        (preset) =>
          preset.passwordProvided &&
          preset.host.toLowerCase() === form.host.toLowerCase() &&
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
  const advanced = ref(!p.host || (form.method !== 'none' && !passwordSet.value))
  const setupNick = ref<HTMLInputElement | null>(null)
  const standardNick = ref<HTMLInputElement | null>(null)
  const alternateNick = ref<HTMLInputElement | null>(null)
  const nickMax = computed(() => nicknameLimit(existing.value?.isupport))
  const nickProblem = computed(() => nicknameError(form.nick, nickMax.value, t))
  const alternateProblem = computed(() =>
    form.altNick ? nicknameError(form.altNick, nickMax.value, t) : '',
  )
  watchEffect(() => {
    setupNick.value?.setCustomValidity(nickProblem.value)
    standardNick.value?.setCustomValidity(nickProblem.value)
    alternateNick.value?.setCustomValidity(alternateProblem.value)
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
    () => !n || form.host.trim() !== n.host || Number(form.port) !== n.port || form.tls !== n.tls,
  )
  watch(
    () => [form.host.trim(), Number(form.port), form.tls, checkAttempt.value] as const,
    ([host, port, tls], _previous, onCleanup) => {
      const controller = new AbortController()
      onCleanup(() => controller.abort())
      if (!needsVerification.value) {
        verification.value = { state: 'verified', detail: '' }
        return
      }
      if (!host || !Number.isInteger(port) || port < 1 || port > 65535) {
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
          if (!controller.signal.aborted)
            verification.value = {
              state: result.reachable ? 'verified' : 'error',
              detail: result.detail,
            }
        } catch (cause) {
          if (!controller.signal.aborted)
            verification.value = { state: 'error', detail: errorText(cause) }
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
    {
      value: 'sasl-external',
      label: t('networkForm.methods.certificate'),
      note: t('networkForm.methods.certificateHelp'),
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
      const bitmap = await createImageBitmap(file)
      const canvas = document.createElement('canvas')
      canvas.width = canvas.height = 96
      const side = Math.min(bitmap.width, bitmap.height)
      canvas
        .getContext('2d')!
        .drawImage(
          bitmap,
          (bitmap.width - side) / 2,
          (bitmap.height - side) / 2,
          side,
          side,
          0,
          0,
          96,
          96,
        )
      bitmap.close()
      const url = canvas.toDataURL('image/webp', 0.85)
      if (url.length > maxIconBytes) throw new Error(t('networkForm.imageTooDetailed'))
      icon.value = url
    } catch (e) {
      error.value = e instanceof Error ? e.message : t('networkForm.imageOpenFailed')
    }
  }

  async function submit() {
    if (needsVerification.value && verification.value.state !== 'verified') return
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
      host: form.host.trim(),
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
      error.value = errorText(e)
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
        error.value = errorText(cause)
      }
    }
  }
  const state = {
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
    nickMax,
    nickProblem,
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
