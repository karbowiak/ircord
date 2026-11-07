<script setup lang="ts">
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
/**
 * Live identity on the current network: nickname, away state and (where the server supports
 * IRCv3 setname) real name. Changes are sent as IRC commands; the backend applies them.
 */
const chat = useChatStore()
const network = computed(() => chat.network)
const nick = ref(network.value?.nick ?? '')
const realname = ref(network.value?.realname ?? '')
const awayMessage = ref('')
const away = computed(() => chat.data?.user.status === 'away')
const nickInput = ref<HTMLInputElement | null>(null)
const nickMax = computed(() => nicknameLimit(network.value?.isupport))
const nickProblem = computed(() => nicknameError(nick.value, nickMax.value))
const error = ref('')
const saving = ref(false)
watchEffect(() => nickInput.value?.setCustomValidity(nickProblem.value))
watch(network, (n) => {
  nick.value = n?.nick ?? ''
  realname.value = n?.realname ?? ''
})
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
  chat.raw(away.value ? 'AWAY' : `AWAY :${awayMessage.value.trim() || 'Away'}`)
}
</script>
<template>
  <section
    v-if="network"
    class="network-identity-settings"
  >
    <h4 class="settings-subtitle">On {{ network.name }}</h4>
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
          :disabled="saving || nick === network.nick"
        >
          Change
        </button>
      </div>
      <p
        id="identity-nick-hint"
        class="field-hint"
        :class="{ 'danger-text': nick && nickProblem }"
      >
        {{ nick && nickProblem ? nickProblem : 'Nickname has no spaces; your display name can.' }}
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
        >Real name / display name</label
      >
      <div>
        <input
          id="identity-realname"
          v-model="realname"
          class="text-input"
          maxlength="100"
          autocomplete="off"
        /><button
          class="button secondary small"
          :disabled="saving || !realname.trim() || realname === network.realname"
        >
          Change
        </button>
      </div>
    </form>
    <p
      v-if="error"
      class="form-error"
      role="alert"
    >
      {{ error }}
    </p>
    <p
      v-else-if="!network.features.setname"
      class="field-hint"
    >
      This network can’t change your real name while connected; edit it in the network settings and
      reconnect.
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
          :placeholder="away ? 'You are away' : 'Away message (optional)'"
          :disabled="away"
          maxlength="200"
        /><button class="button secondary small">
          {{ away ? 'I’m back' : 'Set away' }}
        </button>
      </div>
    </form>
  </section>
</template>
