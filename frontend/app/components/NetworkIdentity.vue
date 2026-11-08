<script setup lang="ts">
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
/**
 * IRC identity per network: nickname, away state and (where the server supports IRCv3 setname)
 * real name. Changes are sent as IRC commands while connected; the backend applies them.
 */
const chat = useChatStore()
const networks = computed(() => chat.data?.networks ?? [])
const selectedId = ref(chat.networkId)
const network = computed(
  () => networks.value.find((n) => n.id === selectedId.value) ?? networks.value[0],
)
const connected = computed(() => network.value?.status === 'connected')
const nick = ref('')
const realname = ref('')
const awayMessage = ref('')
const away = computed(() => chat.data?.user.status === 'away')
const nickInput = ref<{ setCustomValidity: (message: string) => void } | null>(null)
const nickMax = computed(() => nicknameLimit(network.value?.isupport))
const nickProblem = computed(() => nicknameError(nick.value, nickMax.value))
const error = ref('')
const saving = ref(false)
watchEffect(() => nickInput.value?.setCustomValidity(nickProblem.value))
watch(
  () => [network.value?.id, network.value?.nick, network.value?.realname],
  () => {
    nick.value = network.value?.nick ?? ''
    realname.value = network.value?.realname ?? ''
  },
  { immediate: true },
)
async function update(body: { nick?: string; realname?: string }) {
  if (!network.value || saving.value) return
  error.value = ''
  saving.value = true
  try {
    await apiClient().updateSelf(network.value.id, body)
  } catch (cause) {
    error.value = errorText(cause)
  } finally {
    saving.value = false
  }
}
function changeNick() {
  if (!nickProblem.value && nick.value !== network.value?.nick) void update({ nick: nick.value })
}
function changeRealname() {
  if (realname.value.trim()) void update({ realname: realname.value.trim() })
}
function toggleAway() {
  if (!network.value) return
  chat.raw(away.value ? 'AWAY' : `AWAY :${awayMessage.value.trim() || 'Away'}`, network.value.id)
}
</script>
<template>
  <section
    v-if="network"
    class="network-identity-settings mt-8"
  >
    <div
      class="identity-heading flex items-center gap-3 border-b [border-bottom-style:solid] border-b-border"
    >
      <SettingsSubtitle> IRC identity </SettingsSubtitle>
      <AppSelect
        v-if="networks.length > 1"
        v-model="selectedId"
        :options="networks.map((n) => ({ value: n.id, label: n.name }))"
        class="identity-network w-auto h-7.5 mb-1.5 min-h-7.5 pt-0 pr-2 pb-0 pl-2.5 text-[13px]"
        aria-label="Network"
      />
    </div>
    <FieldHint>
      {{
        connected
          ? `What other IRC users see on ${network.name}. Changes apply right away.`
          : `Connect to ${network.name} to change these. Its saved defaults are in the network settings.`
      }}
    </FieldHint>
    <FieldHint
      class="network-account"
      role="status"
    >
      IRC account:
      <strong>{{ connected ? network.account || 'Not signed in' : 'Disconnected' }}</strong>
    </FieldHint>
    <fieldset
      class="identity-fields border-0 m-0 p-0 min-w-0"
      :disabled="!connected || saving"
    >
      <form
        class="inline-field"
        @submit.prevent="changeNick"
      >
        <FieldLabel for="identity-nick">Nickname</FieldLabel>
        <div>
          <UiInput
            id="identity-nick"
            ref="nickInput"
            v-model="nick"
            :maxlength="nickMax"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            required
            :aria-invalid="!!nick && !!nickProblem"
            aria-describedby="identity-nick-hint"
          /><UiButton
            variant="secondary"
            size="sm"
            :disabled="nick === network.nick"
          >
            Change
          </UiButton>
        </div>
        <FieldHint
          id="identity-nick-hint"
          :class="{ 'danger-text': nick && nickProblem }"
        >
          {{ nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
        </FieldHint>
      </form>
      <form
        v-if="network.features.setname"
        class="inline-field"
        @submit.prevent="changeRealname"
      >
        <FieldLabel for="identity-realname">Real name</FieldLabel>
        <div>
          <UiInput
            id="identity-realname"
            v-model="realname"
            maxlength="100"
            autocomplete="off"
            aria-describedby="identity-realname-hint"
          /><UiButton
            variant="secondary"
            size="sm"
            :disabled="!realname.trim() || realname === network.realname"
          >
            Change
          </UiButton>
        </div>
        <FieldHint id="identity-realname-hint">
          {{
            network.features.metadata?.includes('display-name')
              ? 'Shown when someone looks you up (WHOIS).'
              : 'Shown in WHOIS, and as your name to other IRCord users here.'
          }}
        </FieldHint>
      </form>
      <FieldHint v-else>
        Real name: {{ network.realname || network.nick }}. This network only changes it on
        reconnect; edit it in the network settings.
      </FieldHint>
      <form
        class="inline-field"
        @submit.prevent="toggleAway"
      >
        <FieldLabel for="identity-away">Away</FieldLabel>
        <div>
          <UiInput
            id="identity-away"
            v-model="awayMessage"
            :placeholder="away ? 'You are away' : 'Message (optional)'"
            :disabled="away"
            maxlength="200"
          /><UiButton
            variant="secondary"
            size="sm"
          >
            {{ away ? 'I’m back' : 'Set away' }}
          </UiButton>
        </div>
      </form>
    </fieldset>
    <UiAlert
      as="p"
      variant="destructive"
      presentation="inline"
      v-if="error"
      class="form-error mt-3"
    >
      {{ error }}
    </UiAlert>
  </section>
</template>
