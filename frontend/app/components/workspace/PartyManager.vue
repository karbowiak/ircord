<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'
import { foldPersonIdentity } from '../../../shared/utils/people'
import { partyName } from '../../../shared/utils/party'
import { errorText } from '../../utils/api'

type Person = components['schemas']['Person']
type EligiblePerson = Person & { servicesAccount: string }
const props = defineProps<{ mode: 'create' | 'details'; conversationId?: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const chat = useChatStore()
const auth = useAuthStore()
const { confirm } = usePrompt()
const conversation = computed(() =>
  chat.data?.conversations.find((c) => c.id === props.conversationId),
)
const party = computed(() => conversation.value?.party)
const capability = computed(() => chat.data?.capabilities.parties)
const network = computed(() =>
  chat.data?.networks.find((n) => n.id === (party.value?.networkId ?? capability.value?.networkId)),
)
const fold = (value: string) => foldPersonIdentity(value, network.value?.isupport?.CASEMAPPING)
const lookup = usePeopleLookup(() => ({
  networkId: capability.value?.networkId ?? '',
  purpose: 'party',
  limit: 20,
}))
const partnerLookup = usePeopleLookup(() => ({
  networkId: capability.value?.networkId ?? '',
  purpose: 'party',
  limit: 20,
}))
const selected = ref<EligiblePerson[]>([])
const resolvingPartner = ref(props.mode === 'create' && !!props.conversationId)
const requiredPartner = ref('')
const partnerAccount = ref('')
const busy = ref(false)
const error = ref('')
const clientId = crypto.randomUUID()
const statusText = computed(() => {
  const state = lookup.state.value
  if (state.error) return errorText(state.error, t)
  if (state.page?.status === 'unavailable') return t('workspaceUi.party.lookupUnavailable')
  if (state.page?.status === 'partial') return t('workspaceUi.party.lookupPartial')
  return ''
})
const reason = (person: Person) =>
  t(`workspaceUi.party.reasons.${person.reason ?? 'identity_unverified'}`)
const results = computed(() =>
  (lookup.state.value.page?.items ?? [])
    .filter((person) => {
      const account = person.servicesAccount && fold(person.servicesAccount)
      return (
        person.backendAccountId !== auth.account?.id &&
        (!account || account !== fold(network.value?.account ?? '')) &&
        !selected.value.some((item) => account && fold(item.servicesAccount) === account) &&
        !party.value?.members.some(
          (item) => account && fold(item.servicesAccount || item.username) === account,
        )
      )
    })
    .map((person) => ({
      value: person.key,
      label: person.displayName || person.servicesAccount || person.nickname || '',
      detail:
        person.eligible && person.servicesAccount
          ? [
              person.servicesAccount,
              ...(person.nicknames ?? (person.nickname ? [person.nickname] : [])).filter(
                (nick) => fold(nick) !== fold(person.servicesAccount!),
              ),
            ].join(' · ')
          : reason(person),
      disabled: !person.eligible || !person.servicesAccount,
    })),
)
const selectedView = computed(() =>
  selected.value.map((person) => ({
    value: person.key,
    label: person.displayName || person.servicesAccount,
  })),
)
const members = computed(
  () =>
    party.value?.members.map((member) => {
      const id = member.id || member.accountId || ''
      const ownAccount = network.value?.account
      return {
        value: id,
        label: member.displayName || member.servicesAccount || member.username,
        detail: member.servicesAccount || member.username,
        avatarUrl: member.avatarUrl,
        owner: id === party.value?.ownerId,
        me:
          (!!auth.account?.id &&
            (member.backendAccountId || member.accountId) === auth.account.id) ||
          (!!ownAccount &&
            !!member.servicesAccount &&
            fold(member.servicesAccount) === fold(ownAccount)),
      }
    }) ?? [],
)
function add(key: string) {
  const item = lookup.state.value.page?.items.find((person) => person.key === key)
  if (!item?.eligible || !item.servicesAccount) return
  if (
    selected.value.length + (party.value?.members.length ?? 1) >=
    (capability.value?.maxMembers ?? 0)
  ) {
    error.value = t('workspaceUi.party.memberLimit', { count: capability.value?.maxMembers ?? 32 })
    return
  }
  selected.value.push({ ...item, servicesAccount: item.servicesAccount })
}
function removeSelection(key: string) {
  if (key !== requiredPartner.value)
    selected.value = selected.value.filter((item) => item.key !== key)
}
async function run(action: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = errorText(cause, t)
  } finally {
    busy.value = false
  }
}
async function create(name: string, description: string) {
  await run(async () => {
    const result = await apiClient().createParty({
      clientId,
      name,
      description,
      servicesAccounts: selected.value.map((item) => item.servicesAccount),
    })
    await chat.openParty(result)
    if (!chat.data?.conversations.some((c) => c.id === result.conversationId)) {
      error.value = t('workspaceUi.party.createdUnavailable')
      return
    }
    emit('close')
  })
}
async function save(name: string, description: string) {
  if (!party.value) return
  await run(async () =>
    chat.putParty(await apiClient().updateParty(party.value!.id, { name, description })),
  )
}
async function invite() {
  if (!party.value) return
  await run(async () => {
    for (const item of [...selected.value]) {
      await apiClient().inviteParty(party.value!.id, item.servicesAccount)
      removeSelection(item.key)
    }
  })
}
async function changeMember(id: string, transfer: boolean) {
  const current = party.value
  const member = current?.members.find((item) => (item.id || item.accountId) === id)
  if (!current || !member) return
  if (
    !(await confirm({
      title: t(transfer ? 'workspaceUi.party.transferConfirm' : 'workspaceUi.party.removeConfirm', {
        name: member.displayName || member.servicesAccount || member.username,
      }),
      confirm: t(transfer ? 'workspaceUi.party.transfer' : 'workspaceUi.party.remove'),
      danger: !transfer,
    }))
  )
    return
  await run(async () =>
    chat.putParty(
      await (transfer
        ? apiClient().transferParty(current.id, id)
        : apiClient().removePartyMember(current.id, id)),
    ),
  )
}
async function leave() {
  const current = party.value
  if (
    !current ||
    !(await confirm({
      title: t('workspaceUi.party.leaveConfirm'),
      confirm: t('workspaceUi.party.leave'),
      danger: true,
    }))
  )
    return
  await run(async () => {
    await apiClient().leaveParty(current.id)
    emit('close')
  })
}
watch(partnerLookup.state, (state) => {
  if (!resolvingPartner.value || state.loading || !partnerAccount.value) return
  if (state.error || state.page?.status === 'unavailable') {
    error.value = state.error ? errorText(state.error, t) : t('workspaceUi.party.lookupUnavailable')
  } else {
    const exact = state.page?.items.find(
      (person) =>
        person.eligible &&
        person.servicesAccount &&
        fold(person.servicesAccount) === fold(partnerAccount.value),
    )
    if (exact?.servicesAccount) {
      requiredPartner.value = exact.key
      selected.value = [
        { ...exact, servicesAccount: exact.servicesAccount },
        ...selected.value.filter(
          (item) => fold(item.servicesAccount) !== fold(exact.servicesAccount!),
        ),
      ]
    } else
      error.value =
        state.page?.status === 'partial'
          ? t('workspaceUi.party.lookupPartial')
          : t('workspaceUi.party.partnerUnavailable')
  }
  resolvingPartner.value = false
})
onMounted(() => {
  if (props.mode !== 'create' || !props.conversationId) return
  if (
    conversation.value?.kind !== 'direct' ||
    conversation.value.networkId !== capability.value?.networkId
  ) {
    error.value = t('workspaceUi.party.partnerUnavailable')
    resolvingPartner.value = false
    return
  }
  const account = chat.directPartner(conversation.value)?.account
  if (!account || account === '*') {
    error.value = t('workspaceUi.party.partnerUnavailable')
    resolvingPartner.value = false
    return
  }
  partnerAccount.value = account
  partnerLookup.search(account)
})
</script>

<template>
  <PartyCreateDialog
    v-if="mode === 'create'"
    :network="network?.name ?? ''"
    :results="results"
    :selected="selectedView"
    :loading="lookup.state.value.loading"
    :busy="busy || resolvingPartner"
    :blocked="!!conversationId && !requiredPartner"
    :locked="requiredPartner"
    :minimum="conversationId ? 2 : 1"
    :max-name-length="capability?.maxNameLength ?? 100"
    :max-description-length="capability?.maxDescriptionLength ?? 500"
    :search-error="statusText"
    :error="error"
    @close="emit('close')"
    @search="lookup.search"
    @add="add"
    @remove="removeSelection"
    @create="create"
  />
  <PartyDetailsDialog
    v-else-if="party"
    :title="partyName(party, auth.account?.id, t('workspaceUi.party.unnamed'))"
    :name="party.name"
    :description="party.description"
    :state="party.state"
    :members="members"
    :can-manage-members="members.some((member) => member.me && member.owner)"
    :results="results"
    :selected="selectedView"
    :loading="lookup.state.value.loading"
    :busy="busy"
    :search-error="statusText"
    :error="error"
    :max-name-length="capability?.maxNameLength ?? 100"
    :max-description-length="capability?.maxDescriptionLength ?? 500"
    @close="emit('close')"
    @save="save"
    @search="lookup.search"
    @add="add"
    @remove-selection="removeSelection"
    @invite="invite"
    @remove-member="changeMember($event, false)"
    @transfer="changeMember($event, true)"
    @leave="leave"
  />
</template>
