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
      <h4
        class="settings-subtitle text-[12px] font-[650] uppercase tracking-[0.05em] text-faint pb-1.5 border-b [border-bottom-style:solid] border-b-border"
      >
        IRC identity
      </h4>
      <AppSelect
        v-if="networks.length > 1"
        v-model="selectedId"
        :options="networks.map((n) => ({ value: n.id, label: n.name }))"
        class="identity-network w-auto h-7.5 mb-1.5 min-h-7.5 pt-0 pr-2 pb-0 pl-2.5 text-[13px]"
        aria-label="Network"
      />
    </div>
    <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
      {{
        connected
          ? `What other IRC users see on ${network.name}. Changes apply right away.`
          : `Connect to ${network.name} to change these. Its saved defaults are in the network settings.`
      }}
    </p>
    <p
      class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5 network-account"
      role="status"
    >
      IRC account:
      <strong>{{ connected ? network.account || 'Not signed in' : 'Disconnected' }}</strong>
    </p>
    <fieldset
      class="identity-fields border-0 m-0 p-0 min-w-0"
      :disabled="!connected || saving"
    >
      <form
        class="inline-field"
        @submit.prevent="changeNick"
      >
        <UiLabel
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="identity-nick"
          >Nickname</UiLabel
        >
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
        <p
          id="identity-nick-hint"
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
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
        <UiLabel
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="identity-realname"
          >Real name</UiLabel
        >
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
        <p
          id="identity-realname-hint"
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
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
        class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
      >
        Real name: {{ network.realname || network.nick }}. This network only changes it on
        reconnect; edit it in the network settings.
      </p>
      <form
        class="inline-field"
        @submit.prevent="toggleAway"
      >
        <UiLabel
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="identity-away"
          >Away</UiLabel
        >
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
