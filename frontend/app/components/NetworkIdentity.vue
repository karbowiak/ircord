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
const nickInput = ref<HTMLInputElement | null>(null)
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
    class="network-identity-settings"
  >
    <div class="identity-heading">
      <h4 class="settings-subtitle">IRC identity</h4>
      <select
        v-if="networks.length > 1"
        v-model="selectedId"
        class="text-input identity-network"
        aria-label="Network"
      >
        <option
          v-for="n in networks"
          :key="n.id"
          :value="n.id"
        >
          {{ n.name }}
        </option>
      </select>
    </div>
    <p class="field-hint">
      {{
        connected
          ? `What other IRC users see on ${network.name}. Changes apply right away.`
          : `Connect to ${network.name} to change these. Its saved defaults are in the network settings.`
      }}
    </p>
    <p
      class="field-hint network-account"
      role="status"
    >
      IRC account:
      <strong>{{ connected ? network.account || 'Not signed in' : 'Disconnected' }}</strong>
    </p>
    <fieldset
      class="identity-fields"
      :disabled="!connected || saving"
    >
      <form
        class="inline-field"
        @submit.prevent="changeNick"
      >
        <label
          class="field-label"
          for="identity-nick"
          >Nickname</label
        >
        <div>
          <input
            id="identity-nick"
            ref="nickInput"
            v-model="nick"
            class="text-input"
            :maxlength="nickMax"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            required
            :aria-invalid="!!nick && !!nickProblem"
            aria-describedby="identity-nick-hint"
          /><button
            class="button secondary small"
            :disabled="nick === network.nick"
          >
            Change
          </button>
        </div>
        <p
          id="identity-nick-hint"
          class="field-hint"
          :class="{ 'danger-text': nick && nickProblem }"
        >
          {{ nick && nickProblem ? nickProblem : 'How people mention you. No spaces.' }}
        </p>
      </form>
      <form
        v-if="network.features.setname"
        class="inline-field"
        @submit.prevent="changeRealname"
      >
        <label
          class="field-label"
          for="identity-realname"
          >Real name</label
        >
        <div>
          <input
            id="identity-realname"
            v-model="realname"
            class="text-input"
            maxlength="100"
            autocomplete="off"
            aria-describedby="identity-realname-hint"
          /><button
            class="button secondary small"
            :disabled="!realname.trim() || realname === network.realname"
          >
            Change
          </button>
        </div>
        <p
          id="identity-realname-hint"
          class="field-hint"
        >
          {{
            network.features.metadata?.includes('display-name')
              ? 'Shown when someone looks you up (WHOIS).'
              : 'Shown in WHOIS, and as your name to other IRCord users here.'
          }}
        </p>
      </form>
      <p
        v-else
        class="field-hint"
      >
        Real name: {{ network.realname || network.nick }}. This network only changes it on
        reconnect; edit it in the network settings.
      </p>
      <form
        class="inline-field"
        @submit.prevent="toggleAway"
      >
        <label
          class="field-label"
          for="identity-away"
          >Away</label
        >
        <div>
          <input
            id="identity-away"
            v-model="awayMessage"
            class="text-input"
            :placeholder="away ? 'You are away' : 'Message (optional)'"
            :disabled="away"
            maxlength="200"
          /><button class="button secondary small">
            {{ away ? 'I’m back' : 'Set away' }}
          </button>
        </div>
      </form>
    </fieldset>
    <p
      v-if="error"
      class="form-error"
      role="alert"
    >
      {{ error }}
    </p>
  </section>
</template>
