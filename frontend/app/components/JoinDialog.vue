<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { ChannelListing } from '../../shared/types/chat'
import { channelListQuery } from '../queries/chat'
const emit = defineEmits<{ close: [] }>()
const chat = useChatStore()
const name = ref('')
const query = ref('')
const error = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(name, (value) => {
  clearTimeout(timer)
  timer = setTimeout(() => (query.value = value.trim().replace(/^#/, '')), 250)
})
onBeforeUnmount(() => clearTimeout(timer))
const list = useQuery(() => channelListQuery({ networkId: chat.networkId, q: query.value }))
const joined = computed(
  () =>
    new Set(
      chat.data?.conversations
        .filter((c) => c.networkId === chat.networkId && c.kind === 'channel')
        .map((c) => `#${c.name}`.toLowerCase()) ?? [],
    ),
)
const exact = computed(() =>
  list.data.value?.items.some((c) => c.name.toLowerCase() === `#${query.value.toLowerCase()}`),
)
async function join(value: string) {
  error.value = ''
  const existing = chat.data?.conversations.find(
    (c) => c.networkId === chat.networkId && `#${c.name}`.toLowerCase() === value.toLowerCase(),
  )
  if (existing) {
    chat.activate(existing.id)
    emit('close')
    return
  }
  try {
    await chat.join.mutateAsync(value)
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  }
}
const label = (c: ChannelListing) =>
  `${c.users.toLocaleString()} ${c.users === 1 ? 'user' : 'users'}`
</script>
<template>
  <AppDialog
    title="Join a channel"
    wide
    @close="emit('close')"
    ><div class="dialog-form">
      <p class="settings-description">
        Browse {{ chat.network?.name }}, or type a name to join or create one.
      </p>
      <form
        class="join-form"
        @submit.prevent="join(`#${name.replace(/^#/, '')}`)"
      >
        <span>#</span
        ><input
          v-model="name"
          class="text-input"
          aria-label="Channel name"
          autofocus
          placeholder="Search or enter a channel name"
          maxlength="50"
          required
        /><button
          class="button primary"
          :disabled="!name.trim() || chat.join.isLoading"
        >
          Join<AppIcon
            name="right"
            :size="15"
          />
        </button>
      </form>
      <p
        v-if="error"
        class="form-error"
        role="alert"
      >
        {{ error }}
      </p>
      <div
        class="join-channel-list"
        :aria-busy="list.isLoading.value"
      >
        <button
          v-if="query && !exact && list.data.value"
          class="join-create"
          @click="join(`#${query}`)"
        >
          <AppIcon
            name="plus"
            :size="20"
          /><span
            ><strong>Create #{{ query }}</strong
            ><small>Nobody is in it yet; you’ll be its operator.</small></span
          >
        </button>
        <button
          v-for="c in list.data.value?.items"
          :key="c.name"
          @click="join(c.name)"
        >
          <AppIcon
            name="hash"
            :size="20"
          /><span
            ><strong>{{ c.name.replace(/^#/, '') }}</strong
            ><small><MessageContent :text="c.topic || 'No topic'" /></small></span
          ><span
            v-if="joined.has(c.name.toLowerCase())"
            class="joined-badge"
            >Joined</span
          ><span class="channel-member-count"
            ><AppIcon
              name="users"
              :size="13"
            />{{ label(c) }}</span
          >
        </button>
        <p
          v-if="list.isLoading.value && !list.data.value"
          class="field-hint"
        >
          Loading channels…
        </p>
        <p
          v-else-if="list.error.value"
          class="form-error"
        >
          {{ errorText(list.error.value) }}
        </p>
        <p
          v-else-if="!list.data.value?.items.length && !query"
          class="field-hint"
        >
          This network didn’t list any channels.
        </p>
      </div>
    </div></AppDialog
  >
</template>
