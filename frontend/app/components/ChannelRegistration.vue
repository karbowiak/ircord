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
}>()
const { confirm } = usePrompt()

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
  query: () => servicesClient.registration(props.conversationId),
  staleTime: 15_000,
})
const info = computed(() => status.data.value)
const unsupported = computed(() => errorStatus(status.error.value) === 501)
const founder = computed(
  () =>
    !!info.value?.account && info.value.founder?.toLowerCase() === info.value.account.toLowerCase(),
)

const access = useQuery({
  key: () => ['channel-access', props.conversationId],
  query: () => servicesClient.access(props.conversationId),
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
    void status.refetch()
    if (info.value?.registered) void access.refetch()
  }
}

const register = () =>
  run(() => servicesClient.register(props.conversationId), `${props.name} is registered to you.`)

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
      () => servicesClient.unregister(props.conversationId),
      `${props.name} is unregistered.`,
    )
}

const newAccount = ref('')
const newRole = ref<ServicesRole>('o')
async function addAccess() {
  const account = newAccount.value.trim()
  if (
    account &&
    (await run(() => servicesClient.setAccess(props.conversationId, account, newRole.value)))
  )
    newAccount.value = ''
}
const changeRole = (entry: AccessEntry, mode: ServicesRole) =>
  run(() => servicesClient.setAccess(props.conversationId, entry.account, mode))
async function removeAccess(entry: AccessEntry) {
  if (
    await confirm({
      title: `Remove ${entry.account}?`,
      body: `They keep any status they have now, but won’t get ${roleLabel(entry.mode).toLowerCase()} status when they rejoin.`,
      confirm: 'Remove',
    })
  )
    await run(() => servicesClient.removeAccess(props.conversationId, entry.account))
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
  if (await run(() => servicesClient.transfer(props.conversationId, account))) {
    transferPending.value = account
    transferTo.value = ''
  }
}
const date = (iso?: string) =>
  iso ? new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' }) : ''
</script>
<template>
  <div class="appearance-settings channel-registration">
    <p
      v-if="status.isLoading.value && !info"
      class="field-hint"
    >
      Asking ChanServ…
    </p>
    <section
      v-else-if="unsupported"
      class="setting-group"
    >
      <h3>Registration</h3>
      <p class="field-hint">
        {{ networkName }} doesn’t have a ChanServ that IRCord can manage. Use
        <code>/msg ChanServ help</code> to see what its services offer.
      </p>
    </section>
    <p
      v-else-if="status.error.value"
      class="form-error"
      role="alert"
    >
      {{ errorText(status.error.value) }}
    </p>
    <template v-else-if="info">
      <p
        v-if="error"
        class="form-error"
        role="alert"
      >
        {{ error }}
      </p>
      <p
        v-else-if="saved"
        class="field-hint"
        role="status"
      >
        {{ saved }}
      </p>
      <section class="setting-group">
        <h3>Registration</h3>
        <template v-if="info.registered">
          <dl class="channel-facts">
            <div>
              <dt>Founder</dt>
              <dd>{{ info.founder }}<template v-if="founder"> (you)</template></dd>
            </div>
            <div v-if="info.registeredAt">
              <dt>Registered</dt>
              <dd>{{ date(info.registeredAt) }}</dd>
            </div>
          </dl>
          <p class="field-hint">
            ChanServ keeps the topic, modes and roles below even when everyone leaves.
          </p>
        </template>
        <template v-else>
          <p class="field-hint">
            {{ name }} isn’t registered. Registering makes you its founder: ChanServ keeps its topic
            and modes when everyone leaves, and gives you and the people you add their status back
            whenever they join.
          </p>
          <div class="registration-action">
            <button
              class="button primary small"
              :disabled="busy || !info.account || !op"
              @click="register"
            >
              Register {{ name }}
            </button>
            <span
              v-if="!info.account"
              class="field-hint"
              >Log in to a {{ networkName }} account to register channels.</span
            >
            <span
              v-else-if="!op"
              class="field-hint"
              >You need operator status (@) in {{ name }} to register it.</span
            >
            <span
              v-else
              class="field-hint"
              >As <strong>{{ info.account }}</strong></span
            >
          </div>
        </template>
      </section>

      <section
        v-if="info.registered"
        class="setting-group"
      >
        <h3>Access</h3>
        <p class="field-hint">
          Status people get automatically when they join, by account name (not nick).
        </p>
        <p
          v-if="accessDenied"
          class="field-hint"
        >
          Only the founder and people with admin or operator access can see this list.
        </p>
        <p
          v-else-if="access.error.value"
          class="form-error"
        >
          {{ errorText(access.error.value) }}
        </p>
        <template v-else>
          <form
            v-if="founder"
            class="mask-add"
            @submit.prevent="addAccess"
          >
            <input
              v-model="newAccount"
              class="text-input"
              placeholder="Account name"
              aria-label="Account to add"
              autocomplete="off"
              autocapitalize="none"
              spellcheck="false"
            /><select
              v-model="newRole"
              class="text-input access-role"
              aria-label="Role to give"
            >
              <option
                v-for="r in roles.slice(1)"
                :key="r.mode"
                :value="r.mode"
              >
                {{ r.label }}
              </option></select
            ><button
              class="button primary small"
              :disabled="busy || !newAccount.trim()"
            >
              Add
            </button>
          </form>
          <p
            v-if="access.isLoading.value && !access.data.value"
            class="field-hint"
          >
            Loading…
          </p>
          <ul
            v-else
            class="mask-list access-list"
          >
            <li
              v-for="entry in access.data.value?.items"
              :key="entry.account"
            >
              <span class="access-account">{{ entry.account }}</span>
              <select
                v-if="founder && entry.account !== info.founder"
                class="text-input access-role"
                :value="entry.mode"
                :aria-label="`Role for ${entry.account}`"
                :disabled="busy"
                @change="
                  changeRole(entry, ($event.target as HTMLSelectElement).value as ServicesRole)
                "
              >
                <option
                  v-for="r in roles.slice(1)"
                  :key="r.mode"
                  :value="r.mode"
                >
                  {{ r.label }}
                </option>
              </select>
              <span
                v-else
                class="mask-meta"
                >{{ roleLabel(entry.mode) }}</span
              >
              <button
                v-if="founder && entry.account !== info.founder"
                class="icon-button"
                :aria-label="`Remove ${entry.account}`"
                :disabled="busy"
                @click="removeAccess(entry)"
              >
                <AppIcon
                  name="trash"
                  :size="15"
                />
              </button>
            </li>
          </ul>
        </template>
      </section>

      <section
        v-if="info.registered && founder"
        class="setting-group"
      >
        <h3>Ownership</h3>
        <form
          class="mask-add"
          @submit.prevent="transfer"
        >
          <input
            v-model="transferTo"
            class="text-input"
            placeholder="New founder’s account"
            aria-label="Transfer to account"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
          /><button
            class="button secondary small"
            :disabled="busy || !transferTo.trim()"
          >
            Transfer
          </button>
        </form>
        <p
          v-if="transferPending"
          class="field-hint"
          role="status"
        >
          Transfer offered to <strong>{{ transferPending }}</strong
          >. They become the founder once they accept it with
          <code>/msg ChanServ TRANSFER accept {{ name }}</code
          >.
        </p>
        <div class="registration-action">
          <button
            class="button secondary small danger-text"
            :disabled="busy"
            @click="unregister"
          >
            Unregister {{ name }}
          </button>
          <span class="field-hint">Deletes the registration so anyone can claim it.</span>
        </div>
      </section>
    </template>
  </div>
</template>
