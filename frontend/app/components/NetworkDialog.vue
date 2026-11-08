<script setup lang="ts">
import type { NetworkInput } from '../../shared/types/chat'
import { maxIconBytes } from '../../shared/preferences'
import { networkPresets, type NetworkPreset } from '../../shared/network-presets'
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
const props = withDefaults(
  defineProps<{ networkId?: string; prefill?: Partial<NetworkInput>; firstConnection?: boolean }>(),
  { networkId: '', prefill: () => ({}), firstConnection: false },
)
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const { confirm } = usePrompt()
const existing = computed(() => chat.data?.networks.find((n) => n.id === props.networkId))
const editing = computed(() => !!existing.value)
const presets = networkPresets(useRuntimeConfig().public.networkPresets)

type Method = NetworkInput['auth']['method']
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
const nickProblem = computed(() => nicknameError(form.nick, nickMax.value))
const alternateProblem = computed(() =>
  form.altNick ? nicknameError(form.altNick, nickMax.value) : '',
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
    let timer: ReturnType<typeof setTimeout> | undefined
    onCleanup(() => {
      clearTimeout(timer)
      controller.abort()
    })
    if (!needsVerification.value) {
      verification.value = { state: 'verified', detail: '' }
      return
    }
    if (!host || !Number.isInteger(port) || port < 1 || port > 65535) {
      verification.value = { state: 'idle', detail: '' }
      return
    }
    verification.value = { state: 'checking', detail: 'Verifying server address…' }
    timer = setTimeout(async () => {
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
  },
  { immediate: true },
)
const methods: { value: Method; label: string; note: string }[] = [
  { value: 'none', label: 'None', note: 'Connect without logging in.' },
  {
    value: 'sasl-plain',
    label: 'SASL',
    note: 'Log in to your services account while connecting (recommended).',
  },
  { value: 'nickserv', label: 'NickServ', note: 'Identify with NickServ after connecting.' },
  {
    value: 'server-password',
    label: 'Server password',
    note: 'For servers or bouncers that need a PASS.',
  },
  {
    value: 'sasl-external',
    label: 'Certificate',
    note: 'SASL EXTERNAL with a client certificate.',
  },
]
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
    if (url.length > maxIconBytes) throw new Error('That image is too detailed. Try a simpler one.')
    icon.value = url
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'That image couldn’t be opened.'
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
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  }
}
async function remove() {
  const network = existing.value
  if (!network) return
  if (
    await confirm({
      title: `Remove ${network.name}?`,
      body: 'This disconnects and deletes its channels, history and saved login.',
      confirm: 'Remove',
      danger: true,
      requireText: network.name,
    })
  ) {
    try {
      await chat.removeNetwork.mutateAsync(network)
      preferences.setIcon(network.id, null)
      emit('close')
    } catch (cause) {
      error.value = errorText(cause)
    }
  }
}
</script>
<template>
  <AppDialog
    :title="
      editing
        ? `Edit ${existing?.name}`
        : firstConnection
          ? 'Your first connection'
          : 'Add a network'
    "
    :wide="!firstConnection || advanced"
    @close="emit('close')"
  >
    <form
      class="dialog-form pt-0 pr-5 pb-5 pl-5 network-form"
      @submit.prevent="submit"
      @invalid.capture="revealInvalidField"
    >
      <template v-if="firstConnection">
        <p class="connection-setup-intro mt-0 mr-0 mb-4.5 ml-0 text-body leading-[1.65]">
          {{ form.name ? `${form.name} is ready to connect.` : 'Let’s connect you to IRC.' }}
          Choose a nickname and an optional display name.
        </p>
        <div class="connection-setup-nick">
          <UiLabel
            class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
            for="setup-nick"
            >Nickname</UiLabel
          >
          <UiInput
            id="setup-nick"
            :ref="(el) => (setupNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
            v-model="form.nick"
            placeholder="your-nick"
            required
            :maxlength="nickMax"
            autocomplete="nickname"
            autocapitalize="none"
            :aria-invalid="!!form.nick && !!nickProblem"
            aria-describedby="network-nick-hint"
            spellcheck="false"
          />
          <p
            id="network-nick-hint"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
            :class="{ 'danger-text': form.nick && nickProblem }"
          >
            {{ form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
          </p>
        </div>
        <div class="connection-setup-nick">
          <UiLabel
            class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
            for="setup-realname"
            >Display name</UiLabel
          >
          <UiInput
            id="setup-realname"
            v-model="form.realname"
            placeholder="How people know you"
            maxlength="100"
            autocomplete="off"
            aria-describedby="setup-realname-hint"
          />
          <p
            id="setup-realname-hint"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            Optional. Shared as your IRC real name. Leave blank to use your nickname.
          </p>
        </div>
        <p
          v-if="form.host"
          class="field-hint connection-setup-summary text-[12px] text-faint block leading-[1.5] mt-4 mr-0 mb-2 ml-0 wrap-anywhere"
        >
          {{ form.host }}:{{ form.port }} · {{ form.tls ? 'TLS' : 'Plaintext'
          }}<template v-if="form.autoJoin"> · {{ form.autoJoin.split('\n').join(', ') }}</template>
          <template v-if="providedPassword"> · Server password provided</template>
        </p>
        <UiButton
          variant="link"
          size="text"
          type="button"
          class="connection-setup-toggle inline-flex items-center gap-1.5 justify-self-start"
          :aria-expanded="advanced"
          aria-controls="network-connection-settings"
          @click="advanced = !advanced"
        >
          <AppIcon
            :name="advanced ? 'down' : 'right'"
            :size="14"
          />Connection settings
        </UiButton>
      </template>
      <div
        id="network-connection-settings"
        v-show="!firstConnection || advanced"
        class="network-connection-settings flow-root"
      >
        <div
          v-if="!editing && presets.length"
          class="network-presets flex gap-1.5 flex-wrap"
        >
          <button
            v-for="preset in presets"
            :key="preset.name"
            type="button"
            @click="selectPreset(preset)"
          >
            <span
              aria-hidden="true"
              :class="`tone-${preset.color}`"
              >{{ preset.initial }}</span
            >{{ preset.name }}
          </button>
        </div>

        <fieldset>
          <legend>Server</legend>
          <div class="network-identity flex items-end gap-3.5">
            <button
              type="button"
              class="network-icon-picker relative shrink-0 w-14 h-14 mt-2.5 rounded-lg grid [place-items:center] text-[18px] font-[650] overflow-visible"
              :class="iconPreview ? '' : `tone-${existing?.color ?? 'lavender'}`"
              :aria-label="iconPreview ? 'Change network icon' : 'Add a network icon'"
              title="Network icon"
              @click="iconInput?.click()"
            >
              <img
                v-if="iconPreview"
                :src="iconPreview"
                alt=""
              /><span v-else>{{ (form.name || '?').slice(0, 2).toUpperCase() }}</span>
              <AppIcon
                name="image"
                :size="14"
                class="icon-picker-badge absolute right-[-4px] bottom-[-4px] p-[3px] w-5 h-5 rounded-full bg-primary text-primary-foreground"
              />
            </button>
            <input
              ref="iconInput"
              type="file"
              class="sr-only"
              accept="image/png,image/jpeg,image/webp,image/gif"
              aria-label="Network icon image"
              @change="chooseIcon"
            />
            <div class="network-name-field flex-1">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-name"
                >Name</UiLabel
              ><UiInput
                id="network-name"
                v-model="form.name"
                placeholder="My community"
                required
                maxlength="40"
              />
              <UiButton
                variant="link"
                size="text"
                v-if="iconPreview"
                type="button"
                @click="icon = null"
              >
                Remove icon
              </UiButton>
            </div>
          </div>
          <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
            <div class="span-2">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-host"
                >Hostname</UiLabel
              ><UiInput
                id="network-host"
                v-model="form.host"
                placeholder="irc.example.org"
                required
                pattern="[a-zA-Z0-9.\-]+"
              />
            </div>
            <div>
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-port"
                >Port</UiLabel
              ><UiInput
                id="network-port"
                v-model.number="form.port"
                type="number"
                min="1"
                max="65535"
                required
              />
            </div>
            <label class="tls-field flex items-center gap-1.5 h-9.5 text-[13px] text-body"
              ><input
                v-model="form.tls"
                type="checkbox"
              /><AppIcon
                name="lock"
                :size="15"
              />TLS</label
            >
          </div>
        </fieldset>

        <fieldset>
          <legend>Identity</legend>
          <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
            <div v-if="!firstConnection">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-nick"
                >Nickname</UiLabel
              ><UiInput
                id="network-nick"
                :ref="
                  (el) => (standardNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)
                "
                v-model="form.nick"
                required
                :maxlength="nickMax"
                autocomplete="off"
                autocapitalize="none"
                spellcheck="false"
                :aria-invalid="!!form.nick && !!nickProblem"
                aria-describedby="network-nick-hint"
              />
              <p
                id="network-nick-hint"
                class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
                :class="{ 'danger-text': form.nick && nickProblem }"
              >
                {{ form.nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
              </p>
            </div>
            <div>
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-alt"
                >If taken, use</UiLabel
              ><UiInput
                id="network-alt"
                :ref="
                  (el) => (alternateNick = (el as { $el: HTMLInputElement } | null)?.$el ?? null)
                "
                v-model="form.altNick"
                :placeholder="`${form.nick || 'nick'}_`"
                :maxlength="nickMax"
                autocomplete="off"
                autocapitalize="none"
                spellcheck="false"
                :aria-invalid="!!alternateProblem"
                aria-describedby="network-alt-hint"
              />
              <p
                v-if="alternateProblem"
                id="network-alt-hint"
                class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5 danger-text"
              >
                {{ alternateProblem }}
              </p>
            </div>
            <div>
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-user"
                >Username</UiLabel
              ><UiInput
                id="network-user"
                v-model="form.username"
                :placeholder="form.nick || 'username'"
                maxlength="32"
                autocomplete="off"
              />
            </div>
            <div v-if="!firstConnection">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-real"
                >Real name</UiLabel
              ><UiInput
                id="network-real"
                v-model="form.realname"
                placeholder="Your nickname when blank"
                maxlength="100"
                autocomplete="off"
              />
              <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
                Shown in chat and WHOIS. Spaces are allowed.
              </p>
            </div>
          </div>
        </fieldset>

        <fieldset>
          <legend>Login</legend>
          <AppSegmented
            v-model="form.method"
            class="auth-methods flex-wrap mt-2.5"
            label="Login method"
            :options="methods"
          />
          <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
            {{ methods.find((m) => m.value === form.method)?.note }}
          </p>
          <div
            v-if="form.method !== 'none' && form.method !== 'sasl-external'"
            class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]"
          >
            <div v-if="form.method === 'sasl-plain'">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-account"
                >Account</UiLabel
              ><UiInput
                id="network-account"
                v-model="form.account"
                :placeholder="form.nick"
                required
                autocomplete="username"
              />
            </div>
            <div :class="{ 'span-2': form.method !== 'sasl-plain' }">
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-password"
                >Password</UiLabel
              ><UiInput
                id="network-password"
                v-model="form.password"
                type="password"
                :placeholder="
                  providedPassword
                    ? 'Provided by server · type to override'
                    : passwordSet && !clearPassword
                      ? 'Saved · type to replace'
                      : ''
                "
                :required="!passwordSet || clearPassword"
                autocomplete="new-password"
              />
              <UiButton
                variant="link"
                size="text"
                v-if="editing && passwordSet && !form.password"
                type="button"
                @click="clearPassword = !clearPassword"
              >
                {{ clearPassword ? 'Keep saved password' : 'Forget saved password' }}
              </UiButton>
            </div>
          </div>
          <p
            v-if="form.method === 'sasl-external'"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            Client certificates are configured on the server side and aren’t available yet.
          </p>
        </fieldset>

        <fieldset>
          <legend>On connect</legend>
          <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
            <div>
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-autojoin"
                >Join these channels</UiLabel
              ><UiTextarea
                id="network-autojoin"
                v-model="form.autoJoin"
                rows="4"
                placeholder="#channel&#10;#private secretkey"
                spellcheck="false"
              />
            </div>
            <div>
              <UiLabel
                class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
                for="network-perform"
                >Then send these commands</UiLabel
              ><UiTextarea
                id="network-perform"
                v-model="form.perform"
                class="mono"
                rows="4"
                placeholder="MODE mynick +x"
                spellcheck="false"
              />
            </div>
          </div>
          <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
            One per line. Commands are raw IRC lines, without a leading slash.
          </p>
        </fieldset>
      </div>

      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        v-if="error"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <p
        v-if="needsVerification && verification.detail"
        class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
        :class="{ 'danger-text': verification.state === 'error' }"
        role="status"
      >
        {{ verification.detail }}
        <UiButton
          variant="link"
          size="text"
          v-if="verification.state === 'error'"
          type="button"
          @click="checkAttempt++"
        >
          Retry verification
        </UiButton>
      </p>
      <div class="form-actions flex justify-end items-center gap-2 mt-5">
        <UiButton
          variant="secondary"
          v-if="editing"
          type="button"
          class="danger-text"
          @click="remove"
        >
          Remove network
        </UiButton>
        <span class="spacer" />
        <UiButton
          variant="secondary"
          type="button"
          @click="emit('close')"
        >
          {{ firstConnection ? 'Set up later' : 'Cancel' }}</UiButton
        ><UiButton :disabled="saving || (needsVerification && verification.state !== 'verified')">
          {{
            verification.state === 'checking'
              ? 'Verifying…'
              : saving
                ? 'Saving…'
                : editing
                  ? 'Save changes'
                  : firstConnection
                    ? 'Connect'
                    : 'Add network'
          }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
