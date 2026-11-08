<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { AccessEntry, ServicesRole } from '../../shared/types/services'

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

const roles: { mode: ServicesRole; label: string }[] = [
  { mode: 'q', label: 'Founder' },
  { mode: 'a', label: 'Admin' },
  { mode: 'o', label: 'Operator' },
  { mode: 'h', label: 'Half-operator' },
  { mode: 'v', label: 'Voice' },
]
const roleLabel = (mode: string) => roles.find((r) => r.mode === mode)?.label ?? `+${mode}`

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
const error = ref('')
const saved = ref('')
/** Runs one services action; errors show inline, and the views refresh afterwards. */
async function run(action: () => Promise<unknown>, done?: string) {
  busy.value = true
  error.value = ''
  saved.value = ''
  try {
    await action()
    saved.value = done ?? ''
    return true
  } catch (cause) {
    error.value = errorText(cause)
    return false
  } finally {
    busy.value = false
    await status.refetch().catch(() => {})
    if (info.value?.registered) await access.refetch().catch(() => {})
  }
}

const register = () =>
  run(
    () => apiClient().registerChannel(props.conversationId),
    `${props.name} is registered to you.`,
  )

async function unregister() {
  if (
    await confirm({
      title: `Unregister ${props.name}?`,
      body: 'Its founder, access list and stored settings are deleted, and anyone can register it again.',
      confirm: 'Unregister',
      requireText: props.name,
      danger: true,
    })
  )
    await run(
      () => apiClient().unregisterChannel(props.conversationId),
      `${props.name} is unregistered.`,
    )
}

const newAccount = ref('')
type GrantedRole = Exclude<ServicesRole, 'q'>
const roleOptions = roles.slice(1).map((r) => ({ value: r.mode as GrantedRole, label: r.label }))
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
      title: `Remove ${entry.account}?`,
      body: `They keep any status they have now, but won’t get ${roleLabel(entry.mode).toLowerCase()} status when they rejoin.`,
      confirm: 'Remove',
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
      title: `Transfer ${props.name} to ${account}?`,
      body: 'Once they accept, they become the founder and you lose founder control.',
      confirm: 'Transfer',
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
    if (!pending) saved.value = 'Channel ownership updated.'
    transferTo.value = ''
  }
}
const accessNote = (account: string) => {
  const entry = access.data.value?.items.find(
    (e) => e.account.toLowerCase() === account.toLowerCase(),
  )
  return entry ? `${roleLabel(entry.mode)} now` : undefined
}
// Another channel's form must never carry over a half-typed target.
watch(
  () => props.conversationId,
  () => {
    newAccount.value = ''
    transferTo.value = ''
    transferPending.value = ''
    error.value = ''
    saved.value = ''
  },
)
const date = (iso?: string) =>
  iso ? new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' }) : ''
</script>
<template>
  <div class="appearance-settings grid gap-6 channel-registration">
    <p
      v-if="status.isLoading.value && !info"
      class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
    >
      Asking ChanServ…
    </p>
    <section
      v-else-if="unsupported"
      class="setting-group grid gap-3.5"
    >
      <h3>Registration</h3>
      <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
        {{ networkName }} doesn’t have a ChanServ that IRCord can manage. Use
        <code>/msg ChanServ help</code> to see what its services offer.
      </p>
    </section>
    <div v-else-if="status.error.value">
      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ errorText(status.error.value) }}
      </UiAlert>
      <UiButton
        variant="secondary"
        size="sm"
        @click="status.refetch()"
      >
        Try again
      </UiButton>
    </div>
    <template v-else-if="info">
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
        v-else-if="saved"
        class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
        role="status"
      >
        {{ saved }}
      </p>
      <section class="setting-group grid gap-3.5">
        <h3>Registration</h3>
        <template v-if="info.registered">
          <dl
            class="channel-facts grid [grid-template-columns:repeat(auto-fill,_minmax(180px,_1fr))] gap-[12px_20px] m-0"
          >
            <div>
              <dt>Founder</dt>
              <dd>{{ info.founder }}<template v-if="founder"> (you)</template></dd>
            </div>
            <div v-if="info.registeredAt">
              <dt>Registered</dt>
              <dd>
                {{ date(info.registeredAt)
                }}<template v-if="info.automatic">, automatically when you created it</template>
              </dd>
            </div>
          </dl>
          <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
            ChanServ keeps the topic, modes and roles below even when everyone leaves.
          </p>
        </template>
        <template v-else>
          <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
            {{ name }} isn’t registered. Registering makes you its founder: ChanServ keeps its topic
            and modes when everyone leaves, and gives you and the people you add their status back
            whenever they join.
          </p>
          <div class="registration-action flex flex-wrap items-center gap-[8px_12px]">
            <UiButton
              size="sm"
              :disabled="busy || !account || !op"
              @click="register"
            >
              Register {{ name }}
            </UiButton>
            <span
              v-if="!account"
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >Log in to a {{ networkName }} account to register channels.</span
            >
            <span
              v-else-if="!op"
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >You need operator status (@) in {{ name }} to register it.</span
            >
            <span
              v-else
              class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
              >As <strong>{{ account }}</strong></span
            >
          </div>
        </template>
      </section>

      <section
        v-if="info.registered"
        class="setting-group grid gap-3.5"
      >
        <h3>Access</h3>
        <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
          Status people get automatically when they join, by account name (not nick).
        </p>
        <p
          v-if="accessDenied"
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
        >
          Only the founder and people with admin or operator access can see this list.
        </p>
        <p
          v-else-if="access.error.value"
          class="form-error text-destructive text-[13px] mt-3"
        >
          {{ errorText(access.error.value) }}
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
              :accounts-tracked="accountsTracked"
              :exclude="[account ?? '']"
              :note="accessNote"
              label="Account to add"
            /><AppSelect
              v-model="newRole"
              :options="roleOptions"
              :disabled="busy"
              class="access-role w-auto flex-none"
              aria-label="Role to give"
            /><UiButton
              size="sm"
              :disabled="busy || !newAccount.trim()"
            >
              Add
            </UiButton>
          </form>
          <p
            v-if="access.isLoading.value && !access.data.value"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            Loading…
          </p>
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
                class="access-role w-auto flex-none"
                :model-value="entry.mode as GrantedRole"
                :options="roleOptions"
                :aria-label="`Role for ${entry.account}`"
                :disabled="busy"
                @change="changeRole(entry, $event)"
              />
              <span
                v-else
                class="mask-meta ml-auto text-[12px] text-faint whitespace-nowrap"
                >{{ roleLabel(entry.mode) }}</span
              >
              <UiButton
                variant="ghost"
                size="icon-sm"
                v-if="founder && entry.account.toLowerCase() !== info.founder?.toLowerCase()"
                :aria-label="`Remove ${entry.account}`"
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
        <h3>Ownership</h3>
        <form
          class="mask-add flex gap-2"
          @submit.prevent="transfer"
        >
          <AccountCombobox
            v-model="transferTo"
            :disabled="busy"
            :people="people"
            :channel="name"
            :accounts-tracked="accountsTracked"
            :exclude="[account ?? '']"
            placeholder="New founder’s account"
            label="Transfer to account"
          /><UiButton
            variant="secondary"
            size="sm"
            :disabled="busy || !transferTo.trim()"
          >
            Transfer
          </UiButton>
        </form>
        <p
          v-if="transferPending"
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          role="status"
        >
          Transfer offered to <strong>{{ transferPending }}</strong
          >. They become the founder once they accept it with
          <code>/msg ChanServ TRANSFER accept {{ name }}</code
          >.
        </p>
        <div class="registration-action flex flex-wrap items-center gap-[8px_12px]">
          <UiButton
            variant="secondary"
            size="sm"
            class="danger-text"
            :disabled="busy"
            @click="unregister"
          >
            Unregister {{ name }}
          </UiButton>
          <span class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
            >Deletes the registration so anyone can claim it.</span
          >
        </div>
      </section>
    </template>
  </div>
</template>
