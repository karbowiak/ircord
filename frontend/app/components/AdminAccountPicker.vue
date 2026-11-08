<script setup lang="ts">
import type { components } from '../../shared/generated/api'

type Match = components['schemas']['IdentityMatch']
type User = NonNullable<Match['user']>

/**
 * Finds an IRCord account by username, or by the nickname someone uses on the IRC server, and
 * emits the account it resolved to. Text alone is never submitted: only a match the backend
 * marked eligible can be chosen. Ineligible matches explain why (not logged in, no IRCord account).
 */
const props = defineProps<{ label: string; action: string; busy?: boolean }>()
const emit = defineEmits<{ choose: [user: User] }>()
const query = ref('')
const matches = ref<Match[]>([])
const active = ref(0)
const error = ref('')
const listId = useId()
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
const refused = useAdministrationRefusal()

watch(query, (value) => {
  clearTimeout(timer)
  controller?.abort()
  error.value = ''
  if (!value.trim()) {
    matches.value = []
    return
  }
  timer = setTimeout(async () => {
    controller = new AbortController()
    try {
      const result = await apiClient().administrationIdentities(value.trim(), controller.signal)
      matches.value = result.items
      active.value = Math.max(
        0,
        result.items.findIndex((m) => m.eligible),
      )
    } catch (e) {
      if ((e as Error).name === 'AbortError') return
      refused(e)
      error.value = errorText(e)
    }
  }, 200)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  controller?.abort()
})
const chosen = computed(() => {
  const match = matches.value[active.value]
  return match?.eligible ? match.user : undefined
})
function choose(index = active.value) {
  const match = matches.value[index]
  if (!match?.eligible || !match.user || props.busy) return
  emit('choose', match.user)
  query.value = ''
}
function keydown(event: KeyboardEvent) {
  const count = matches.value.length
  if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && count) {
    event.preventDefault()
    active.value = (active.value + (event.key === 'ArrowDown' ? 1 : count - 1)) % count
  } else if (event.key === 'Enter') {
    event.preventDefault()
    choose()
  }
}
const describe = (m: Match) =>
  m.nickname && m.account && m.nickname.toLowerCase() !== m.account.toLowerCase()
    ? `${m.nickname} → ${m.account}`
    : (m.account ?? m.nickname ?? '')
</script>
<template>
  <div class="admin-picker">
    <div class="admin-picker-row">
      <input
        v-model="query"
        class="text-input"
        type="search"
        role="combobox"
        autocomplete="off"
        spellcheck="false"
        maxlength="64"
        :aria-label="label"
        :placeholder="label"
        :aria-expanded="matches.length > 0"
        :aria-controls="matches.length ? listId : undefined"
        aria-autocomplete="list"
        :aria-activedescendant="matches.length ? `${listId}-${active}` : undefined"
        @keydown="keydown"
      />
      <button
        class="button"
        :disabled="!chosen || busy"
        @click="choose()"
      >
        {{ action }}
      </button>
    </div>
    <p
      v-if="error"
      class="form-error"
      role="alert"
    >
      {{ error }}
    </p>
    <ul
      v-if="matches.length"
      :id="listId"
      class="admin-picker-list"
      role="listbox"
      :aria-label="label"
    >
      <li
        v-for="(match, index) in matches"
        :id="`${listId}-${index}`"
        :key="index"
        role="option"
        :aria-selected="index === active"
        :aria-disabled="!match.eligible"
        :class="{ active: index === active, unavailable: !match.eligible }"
        @mousedown.prevent
        @mousemove="active = index"
        @click="choose(index)"
      >
        <strong>{{ describe(match) }}</strong>
        <small v-if="match.note">{{ match.note }}</small>
        <small v-else-if="match.user?.owner">Already an owner</small>
      </li>
    </ul>
    <p
      v-else-if="query.trim() && !error"
      class="field-hint"
    >
      No IRCord account matches “{{ query.trim() }}”.
    </p>
  </div>
</template>
