<script setup vapor lang="ts">
import { useQuery } from '@pinia/colada'
import type { AccessEntry, ServicesRole } from '../../../shared/types/services'

/**
 * A channel's ChanServ registration: who founded it, the persistent roles granted by account,
 * and transferring or dropping it. Networks without a supported ChanServ get the raw-command
 * hint instead; actions you aren't allowed to take are explained rather than hidden.
 */
const props = defineProps<{
  conversationId: string
  name: string
  networkName: string
  op: boolean
  account?: string
}>()
const { confirm } = usePrompt()
const chat = useChatStore()
const { t } = useI18n()
const errorMessage = (value: unknown) => errorText(value, (key) => t(key))

// People in the channel, for account suggestions. Only accounts the network reports are offered.
const conversation = computed(() =>
  chat.data?.conversations.find((x) => x.id === props.conversationId),
)
const people = computed(() =>
  (conversation.value?.memberIds ?? [])
    .filter((id) => id !== 'you')
    .map((id) => chat.member(id))
    .filter((m) => !!m),
)
const accountsTracked = computed(() => {
  const c = conversation.value
  return !!c && !!chat.networkOf(c)?.capabilities.includes('account-notify')
})

const roles = computed<{ mode: ServicesRole; label: string }[]>(() => [
  { mode: 'q', label: t('collectionsUi.channel.registration.founder') },
  { mode: 'a', label: t('collectionsUi.channel.registration.admin') },
  { mode: 'o', label: t('collectionsUi.channel.registration.operator') },
  { mode: 'h', label: t('collectionsUi.channel.registration.halfOperator') },
  { mode: 'v', label: t('collectionsUi.channel.registration.voice') },
])
const roleLabel = (mode: string) =>
  roles.value.find((role) => role.mode === mode)?.label ??
  t('collectionsUi.channel.registration.roleFallback', { mode })

const status = useQuery({
  key: () => ['channel-registration', props.conversationId],
  query: () => apiClient().channelRegistration(props.conversationId),
  staleTime: 15_000,
})
const info = computed(() => status.data.value)
const unsupported = computed(() => errorStatus(status.error.value) === 501)
const founder = computed(
  () => !!props.account && info.value?.founder?.toLowerCase() === props.account.toLowerCase(),
)

const access = useQuery({
  key: () => ['channel-access', props.conversationId],
  query: () => apiClient().channelAccess(props.conversationId),
  enabled: () => !!info.value?.registered,
  staleTime: 15_000,
})
const accessDenied = computed(() => errorStatus(access.error.value) === 403)

const busy = ref(false)
const error = ref<unknown>()
const saved = ref<{ key: string; values?: Record<string, string | number> } | null>(null)
/** Runs one services action; errors show inline, and the views refresh afterwards. */
async function run(
  action: () => Promise<unknown>,
  done?: { key: string; values?: Record<string, string | number> },
) {
  busy.value = true
  error.value = undefined
  saved.value = null
  try {
    await action()
    saved.value = done ?? null
    return true
  } catch (cause) {
    error.value = cause
    return false
  } finally {
    busy.value = false
    await status.refetch().catch(() => {})
    if (info.value?.registered) await access.refetch().catch(() => {})
  }
}

const register = () =>
  run(() => apiClient().registerChannel(props.conversationId), {
    key: 'collectionsUi.channel.registration.registeredStatus',
    values: { channel: props.name },
  })

async function unregister() {
  if (
    await confirm({
      title: t('collectionsUi.channel.registration.unregisterConfirm', { channel: props.name }),
      body: t('collectionsUi.channel.registration.unregisterWarning'),
      confirm: t('collectionsUi.channel.registration.unregister'),
      requireText: props.name,
      danger: true,
    })
  )
    await run(() => apiClient().unregisterChannel(props.conversationId), {
      key: 'collectionsUi.channel.registration.unregisteredStatus',
      values: { channel: props.name },
    })
}

const newAccount = ref('')
type GrantedRole = Exclude<ServicesRole, 'q'>
const roleOptions = computed(() =>
  roles.value.slice(1).map((role) => ({ value: role.mode as GrantedRole, label: role.label })),
)
const newRole = ref<GrantedRole>('o')
async function addAccess() {
  const account = newAccount.value.trim()
  if (
    account &&
    (await run(() => apiClient().setChannelAccess(props.conversationId, account, newRole.value)))
  )
    newAccount.value = ''
}
const changeRole = (entry: AccessEntry, mode: GrantedRole) =>
  run(() => apiClient().setChannelAccess(props.conversationId, entry.account, mode))
async function removeAccess(entry: AccessEntry) {
  if (
    await confirm({
      title: t('collectionsUi.channel.registration.removeAccessConfirm', {
        account: entry.account,
      }),
      body: t('collectionsUi.channel.registration.removeAccessWarning', {
        role: roleLabel(entry.mode).toLowerCase(),
      }),
      confirm: t('common.remove'),
    })
  )
    await run(() => apiClient().removeChannelAccess(props.conversationId, entry.account))
}

const transferTo = ref('')
const transferPending = ref('')
async function transfer() {
  const account = transferTo.value.trim()
  if (
    !account ||
    !(await confirm({
      title: t('collectionsUi.channel.registration.transferConfirm', {
        channel: props.name,
        account,
      }),
      body: t('collectionsUi.channel.registration.transferWarning'),
      confirm: t('collectionsUi.channel.registration.transfer'),
      danger: true,
    }))
  )
    return
  let pending = false
  if (
    await run(async () => {
      pending =
        (await apiClient().transferChannel(props.conversationId, account)).status === 'pending'
    })
  ) {
    transferPending.value = pending ? account : ''
    if (!pending) saved.value = { key: 'collectionsUi.channel.registration.ownershipUpdated' }
    transferTo.value = ''
  }
}
const accessNote = (account: string) => {
  const entry = access.data.value?.items.find(
    (e) => e.account.toLowerCase() === account.toLowerCase(),
  )
  return entry
    ? t('collectionsUi.channel.registration.roleNow', {
        role: roleLabel(entry.mode).toLowerCase(),
      })
    : undefined
}
// Another channel's form must never carry over a half-typed target.
watch(
  () => props.conversationId,
  () => {
    newAccount.value = ''
    transferTo.value = ''
    transferPending.value = ''
    error.value = undefined
    saved.value = null
  },
)
const date = (iso?: string) =>
  iso ? new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' }) : ''
</script>
<template>
  <div class="appearance-settings grid gap-6 channel-registration">
    <FieldHint v-if="status.isLoading.value && !info">{{
      t('collectionsUi.channel.registration.askingChanServ')
    }}</FieldHint>
    <section
      v-else-if="unsupported"
      class="setting-group grid gap-3.5"
    >
      <h3>{{ t('collectionsUi.channel.registration.registration') }}</h3>
      <FieldHint>
        {{ t('collectionsUi.channel.registration.unsupported', { network: networkName }) }}
        {{
          t('collectionsUi.channel.registration.unsupportedHelp', { command: '/msg ChanServ help' })
        }}
      </FieldHint>
    </section>
    <div v-else-if="status.error.value">
      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ errorMessage(status.error.value) }}
      </UiAlert>
      <UiButton
        variant="secondary"
        size="sm"
        @click="status.refetch()"
      >
        {{ t('common.tryAgain') }}
      </UiButton>
    </div>
    <template v-else-if="info">
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ errorMessage(error) }}
      </UiAlert>
      <FieldHint
        v-else-if="saved"
        role="status"
      >
        {{ t(saved.key, saved.values ?? {}) }}
      </FieldHint>
      <section class="setting-group grid gap-3.5">
        <h3>{{ t('collectionsUi.channel.registration.registration') }}</h3>
        <template v-if="info.registered">
          <dl class="channel-facts stat-strip">
            <div>
              <dt>{{ t('collectionsUi.channel.registration.founder') }}</dt>
              <dd>
                {{
                  t(
                    founder
                      ? 'collectionsUi.channel.registration.founderYou'
                      : 'collectionsUi.channel.registration.founderAccount',
                    { account: info.founder },
                  )
                }}
              </dd>
            </div>
            <div v-if="info.registeredAt">
              <dt>{{ t('collectionsUi.channel.registration.registered') }}</dt>
              <dd>
                {{
                  t(
                    info.automatic
                      ? 'collectionsUi.channel.registration.registeredAutomatically'
                      : 'collectionsUi.channel.registration.registeredDate',
                    { date: date(info.registeredAt) },
                  )
                }}
              </dd>
            </div>
          </dl>
          <FieldHint>
            {{ t('collectionsUi.channel.registration.persistenceHelp') }}
          </FieldHint>
        </template>
        <template v-else>
          <FieldHint>
            {{ t('collectionsUi.channel.registration.notRegistered', { channel: name }) }}
          </FieldHint>
          <div
            class="registration-action flex flex-wrap items-center gap-[8px_12px] [&_.field-hint]:m-0"
          >
            <UiButton
              size="sm"
              :disabled="busy || !account || !op"
              @click="register"
            >
              {{ t('collectionsUi.channel.registration.register', { channel: name }) }}
            </UiButton>
            <FieldHint
              v-if="!account"
              as="span"
              >{{
                t('collectionsUi.channel.registration.loginToRegister', { network: networkName })
              }}</FieldHint
            >
            <FieldHint
              v-else-if="!op"
              as="span"
              >{{
                t('collectionsUi.channel.registration.operatorToRegister', {
                  prefix: '@',
                  channel: name,
                })
              }}</FieldHint
            >
            <FieldHint
              v-else
              as="span"
              >{{ t('collectionsUi.channel.registration.asAccount', { account }) }}</FieldHint
            >
          </div>
        </template>
      </section>

      <section
        v-if="info.registered"
        class="setting-group grid gap-3.5"
      >
        <h3>{{ t('collectionsUi.channel.registration.access') }}</h3>
        <FieldHint>
          {{ t('collectionsUi.channel.registration.accessHelp') }}
        </FieldHint>
        <FieldHint v-if="accessDenied">
          {{ t('collectionsUi.channel.registration.accessDenied') }}
        </FieldHint>
        <p
          v-else-if="access.error.value"
          class="form-error text-destructive text-[13px] mt-3"
        >
          {{ errorMessage(access.error.value) }}
        </p>
        <template v-else>
          <form
            v-if="founder"
            class="mask-add flex gap-2"
            @submit.prevent="addAccess"
          >
            <AccountCombobox
              v-model="newAccount"
              :disabled="busy"
              :people="people"
              :channel="name"
              :network-id="conversation?.networkId ?? ''"
              :conversation-id="conversationId"
              :case-mapping="
                conversation ? chat.networkOf(conversation)?.isupport?.CASEMAPPING : undefined
              "
              :accounts-tracked="accountsTracked"
              :exclude="[account ?? '']"
              :note="accessNote"
              :label="t('collectionsUi.channel.registration.accountToAdd')"
            /><AppSelect
              v-model="newRole"
              :options="roleOptions"
              :disabled="busy"
              class="access-role w-auto flex-none"
              :aria-label="t('collectionsUi.channel.registration.roleToGive')"
            /><UiButton
              size="sm"
              :disabled="busy || !newAccount.trim()"
            >
              {{ t('collectionsUi.channel.registration.add') }}
            </UiButton>
          </form>
          <FieldHint v-if="access.isLoading.value && !access.data.value">{{
            t('collectionsUi.channel.registration.loadingAccess')
          }}</FieldHint>
          <ul
            v-else
            class="mask-list list-none m-0 p-0 grid gap-0.5 access-list"
          >
            <li
              v-for="entry in access.data.value?.items"
              :key="entry.account"
            >
              <span class="access-account font-[600] wrap-anywhere">{{ entry.account }}</span>
              <AppSelect
                v-if="founder && entry.account.toLowerCase() !== info.founder?.toLowerCase()"
                class="access-role w-auto flex-none ml-auto"
                :model-value="entry.mode as GrantedRole"
                :options="roleOptions"
                :aria-label="
                  t('collectionsUi.channel.registration.roleFor', { account: entry.account })
                "
                :disabled="busy"
                @change="changeRole(entry, $event)"
              />
              <span
                v-else
                class="mask-meta ml-auto text-[12px] text-faint whitespace-nowrap"
                >{{ roleLabel(entry.mode) }}</span
              >
              <UiButton
                v-if="founder && entry.account.toLowerCase() !== info.founder?.toLowerCase()"
                variant="ghost"
                size="icon-sm"
                :aria-label="
                  t('collectionsUi.channel.registration.removeAccount', { account: entry.account })
                "
                :disabled="busy"
                @click="removeAccess(entry)"
              >
                <AppIcon
                  name="trash"
                  :size="15"
                />
              </UiButton>
            </li>
          </ul>
        </template>
      </section>

      <section
        v-if="info.registered && founder"
        class="setting-group grid gap-3.5"
      >
        <h3>{{ t('collectionsUi.channel.registration.ownership') }}</h3>
        <form
          class="mask-add flex gap-2"
          @submit.prevent="transfer"
        >
          <AccountCombobox
            v-model="transferTo"
            :disabled="busy"
            :people="people"
            :channel="name"
            :network-id="conversation?.networkId ?? ''"
            :conversation-id="conversationId"
            :case-mapping="
              conversation ? chat.networkOf(conversation)?.isupport?.CASEMAPPING : undefined
            "
            :accounts-tracked="accountsTracked"
            :exclude="[account ?? '']"
            :placeholder="t('collectionsUi.channel.registration.newFounderAccount')"
            :label="t('collectionsUi.channel.registration.transferToAccount')"
          /><UiButton
            variant="secondary"
            size="sm"
            :disabled="busy || !transferTo.trim()"
          >
            {{ t('collectionsUi.channel.registration.transfer') }}
          </UiButton>
        </form>
        <FieldHint
          v-if="transferPending"
          role="status"
        >
          {{
            t('collectionsUi.channel.registration.transferOffered', { account: transferPending })
          }}
          {{ t('collectionsUi.channel.registration.transferAcceptHint') }}
          <code>/msg ChanServ TRANSFER accept {{ name }}</code
          >.
        </FieldHint>
        <div
          class="registration-action flex flex-wrap items-center gap-[8px_12px] [&_.field-hint]:m-0"
        >
          <UiButton
            variant="secondary-destructive"
            size="sm"
            :disabled="busy"
            @click="unregister"
          >
            {{ t('collectionsUi.channel.registration.unregisterChannel', { channel: name }) }}
          </UiButton>
          <FieldHint as="span">{{
            t('collectionsUi.channel.registration.unregisterHelp')
          }}</FieldHint>
        </div>
      </section>
    </template>
  </div>
</template>
