<script setup vapor lang="ts">
import {
  accountSuggestions,
  nickWithOtherAccount,
  type AccountSuggestion,
  type RosterPerson,
} from '../../../shared/utils/account-suggestions'
import { foldPersonIdentity } from '../../../shared/utils/people'

const props = withDefaults(
  defineProps<{
    people: readonly RosterPerson[]
    channel: string
    label: string
    placeholder?: string
    disabled?: boolean
    accountsTracked?: boolean
    exclude?: readonly string[]
    note?: (account: string) => string | undefined
    caseMapping?: string
    networkId: string
    conversationId: string
  }>(),
  { placeholder: undefined, disabled: false, accountsTracked: true, exclude: () => [] },
)
const model = defineModel<string>({ required: true })
const { t, n } = useI18n()
const hintId = useId()
const lookup = usePeopleLookup(() => ({
  networkId: props.networkId,
  conversationId: props.conversationId,
  purpose: 'services',
  limit: 20,
}))
const networkHint = computed(() => t('collectionsUi.accountPicker.networkHint'))
const results = computed(() =>
  accountSuggestions(props.people, model.value, {
    exclude: props.exclude,
    caseMapping: props.caseMapping,
  }),
)
const accounts = computed(() => {
  const fold = (value: string) => foldPersonIdentity(value, props.caseMapping)
  const found = new Set(results.value.accounts.map((item) => fold(item.account)))
  const excluded = new Set(props.exclude.map(fold))
  const extra: AccountSuggestion[] = []
  for (const person of lookup.state.value.page?.items ?? []) {
    const account = person.servicesAccount
    if (!account || !person.eligible || found.has(fold(account)) || excluded.has(fold(account)))
      continue
    found.add(fold(account))
    extra.push({
      account,
      nicks: person.nicknames ?? (person.nickname ? [person.nickname] : []),
      name: person.displayName !== account ? person.displayName : undefined,
    })
  }
  return [...results.value.accounts, ...extra].slice(0, 8)
})
const options = computed(() => [
  ...accounts.value.map((account) => ({
    key: `account:${foldPersonIdentity(account.account, props.caseMapping)}`,
    label: account.account,
  })),
  ...results.value.unknown.map((person) => ({
    key: `unknown:${foldPersonIdentity(person.nick, props.caseMapping)}`,
    label: person.nick,
    disabled: true,
  })),
])
const accountFor = (key: string) =>
  accounts.value.find(
    (item) => `account:${foldPersonIdentity(item.account, props.caseMapping)}` === key,
  )
const inChannel = (item: AccountSuggestion) =>
  results.value.accounts.some(
    (account) =>
      foldPersonIdentity(account.account, props.caseMapping) ===
      foldPersonIdentity(item.account, props.caseMapping),
  )
const others = (item: AccountSuggestion) =>
  item.nicks.filter(
    (nick) =>
      foldPersonIdentity(nick, props.caseMapping) !==
      foldPersonIdentity(item.account, props.caseMapping),
  )
function choose(key: string) {
  const account = accountFor(key)
  if (account) model.value = account.account
}
const typed = computed(() => model.value.trim())
const exact = computed(() =>
  accounts.value.find(
    (item) =>
      foldPersonIdentity(item.account, props.caseMapping) ===
      foldPersonIdentity(typed.value, props.caseMapping),
  ),
)
const mismatch = computed(() =>
  exact.value ? undefined : nickWithOtherAccount(props.people, typed.value, props.caseMapping),
)
</script>

<template>
  <div class="account-combobox flex-1 min-w-0 grid gap-1 max-[700px]:basis-full">
    <PeopleCombobox
      v-model="model"
      :options="options"
      :label="label"
      :placeholder="placeholder ?? t('collectionsUi.accountPicker.accountName')"
      :disabled="disabled"
      :described-by="hintId"
      popup
      @select="choose"
      @search="lookup.search"
    >
      <template #option="{ option }">
        <span
          class="account-option flex flex-col items-start justify-center gap-[1px] min-h-10 wrap-anywhere"
        >
          <span class="font-[550] text-foreground">{{ option.label }}</span>
          <span
            v-if="accountFor(option.key)"
            class="text-[0.82em] text-muted-foreground"
          >
            <template v-if="others(accountFor(option.key)!).length">{{
              t(
                inChannel(accountFor(option.key)!)
                  ? 'collectionsUi.accountPicker.inChannelAs'
                  : 'collectionsUi.accountPicker.aliases',
                {
                  channel,
                  nicks: others(accountFor(option.key)!).join(', '),
                },
              )
            }}</template>
            <template v-else-if="accountFor(option.key)!.nicks.length > 1">{{
              t(
                'collectionsUi.accountPicker.sessions',
                { count: n(accountFor(option.key)!.nicks.length) },
                accountFor(option.key)!.nicks.length,
              )
            }}</template>
            <template v-if="accountFor(option.key)!.name">
              · {{ accountFor(option.key)!.name }}</template
            >
            <template v-if="note?.(option.label)"> · {{ note(option.label) }}</template>
          </span>
          <span
            v-else
            class="text-[0.82em] text-muted-foreground"
            >{{
              t(
                accountsTracked
                  ? 'collectionsUi.accountPicker.notLoggedIn'
                  : 'collectionsUi.accountPicker.notReported',
              )
            }}</span
          >
        </span>
      </template>
      <template #default="{ open }">
        <FieldHint
          v-if="
            open && (lookup.state.value.error || lookup.state.value.page?.status === 'unavailable')
          "
          >{{ t('collectionsUi.accountPicker.lookupUnavailable') }}</FieldHint
        >
        <FieldHint v-else-if="open && lookup.state.value.page?.status === 'partial'">{{
          t('collectionsUi.accountPicker.lookupPartial')
        }}</FieldHint>
      </template>
    </PeopleCombobox>
    <FieldHint
      :id="hintId"
      class="account-target m-0 [&_strong]:font-[600] [&_strong]:text-foreground [&_strong]:wrap-anywhere"
      aria-live="polite"
    >
      <template v-if="!typed"
        ><template v-if="people.length">{{
          t('collectionsUi.accountPicker.typeAccountOrNick', { channel })
        }}</template
        ><template v-else>{{ t('collectionsUi.accountPicker.typeAccount') }}</template></template
      >
      <template v-else-if="mismatch?.account">{{
        t('collectionsUi.accountPicker.nickAccountMismatch', {
          nick: mismatch.nick,
          account: mismatch.account,
        })
      }}</template>
      <template v-else-if="mismatch && accountsTracked">{{
        t('collectionsUi.accountPicker.nickNotLoggedIn', { nick: mismatch.nick, account: typed })
      }}</template>
      <template v-else-if="mismatch">{{
        t('collectionsUi.accountPicker.nickAccountUnverified', {
          hint: networkHint,
          account: typed,
          nick: mismatch.nick,
        })
      }}</template>
      <template v-else-if="exact && inChannel(exact) && exact.nicks.length">{{
        t('collectionsUi.accountPicker.exactAccount', {
          account: exact.account,
          nicks: exact.nicks.join(', '),
          channel,
        })
      }}</template>
      <template v-else>{{
        t('collectionsUi.accountPicker.typedAccount', { account: typed, channel })
      }}</template>
    </FieldHint>
  </div>
</template>
