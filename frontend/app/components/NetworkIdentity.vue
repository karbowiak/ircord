<script setup vapor lang="ts">
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
/**
 * IRC identity per network: nickname, away state and (where the server supports IRCv3 setname)
 * real name. Changes are sent as IRC commands while connected; the backend applies them.
 */
const chat = useChatStore()
const { t } = useI18n()
const $t = t
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
const nickProblem = computed(() => nicknameError(nick.value, nickMax.value, t))
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
    error.value = errorText(cause, (key) => t(key))
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
      <SettingsSubtitle class="flex-1 border-b-0">
        {{ $t('networkIdentity.title') }}
      </SettingsSubtitle>
      <AppSelect
        v-if="networks.length > 1"
        v-model="selectedId"
        :options="networks.map((n) => ({ value: n.id, label: n.name }))"
        class="identity-network w-auto h-7.5 mb-1.5 min-h-7.5 pt-0 pr-2 pb-0 pl-2.5 text-[13px]"
        :aria-label="$t('profileDialog.network')"
      />
    </div>
    <FieldHint>
      {{
        connected
          ? $t('networkIdentity.connectedHelp', { name: network.name })
          : $t('networkIdentity.disconnectedHelp', { name: network.name })
      }}
    </FieldHint>
    <FieldHint
      class="network-account"
      role="status"
    >
      <i18n-t
        keypath="networkIdentity.account"
        tag="span"
        ><template #account
          ><strong>{{
            connected
              ? network.account || $t('networkIdentity.notSignedIn')
              : $t('networkIdentity.disconnected')
          }}</strong></template
        ></i18n-t
      >
    </FieldHint>
    <fieldset
      class="identity-fields border-0 m-0 p-0 min-w-0 disabled:opacity-60 [&_.field-label]:mt-3"
      :disabled="!connected || saving"
    >
      <form
        class="inline-field"
        @submit.prevent="changeNick"
      >
        <AppField
          id="identity-nick"
          :label="$t('networkForm.nickname')"
          :help="nick && nickProblem ? nickProblem : $t('networkForm.nicknameHelp')"
          :help-class="{ 'text-destructive': nick && nickProblem }"
        >
          <template #default="control">
            <div class="flex gap-2 [&_[data-slot=button]]:h-auto">
              <UiInput
                v-bind="control"
                ref="nickInput"
                v-model="nick"
                :maxlength="nickMax"
                autocomplete="off"
                autocapitalize="none"
                spellcheck="false"
                required
                :aria-invalid="!!nick && !!nickProblem"
              /><UiButton
                variant="secondary"
                size="sm"
                :disabled="nick === network.nick"
              >
                {{ $t('networkIdentity.change') }}
              </UiButton>
            </div>
          </template>
        </AppField>
      </form>
      <form
        v-if="network.features.setname"
        class="inline-field"
        @submit.prevent="changeRealname"
      >
        <AppField
          id="identity-realname"
          :label="$t('networkForm.realName')"
          :help="
            network.features.metadata?.includes('display-name')
              ? $t('networkIdentity.realNameWhois')
              : $t('networkIdentity.realNameShared')
          "
        >
          <template #default="control">
            <div class="flex gap-2 [&_[data-slot=button]]:h-auto">
              <UiInput
                v-bind="control"
                v-model="realname"
                maxlength="100"
                autocomplete="off"
              /><UiButton
                variant="secondary"
                size="sm"
                :disabled="!realname.trim() || realname === network.realname"
              >
                {{ $t('networkIdentity.change') }}
              </UiButton>
            </div>
          </template>
        </AppField>
      </form>
      <FieldHint v-else>
        {{ $t('networkIdentity.realNameReconnect', { name: network.realname || network.nick }) }}
      </FieldHint>
      <form
        class="inline-field"
        @submit.prevent="toggleAway"
      >
        <AppField
          id="identity-away"
          :label="$t('networkIdentity.away')"
        >
          <template #default="control">
            <div class="flex gap-2 [&_[data-slot=button]]:h-auto">
              <UiInput
                v-bind="control"
                v-model="awayMessage"
                :placeholder="
                  away
                    ? chat.data?.user.awayMessage || $t('networkIdentity.youAreAway')
                    : $t('networkIdentity.awayMessage')
                "
                :disabled="away"
                maxlength="200"
              /><UiButton
                variant="secondary"
                size="sm"
              >
                {{ away ? $t('networkIdentity.back') : $t('networkIdentity.setAway') }}
              </UiButton>
            </div>
          </template>
        </AppField>
      </form>
    </fieldset>
    <UiAlert
      v-if="error"
      as="p"
      variant="destructive"
      presentation="inline"
      class="form-error mt-3"
    >
      {{ error }}
    </UiAlert>
  </section>
</template>
