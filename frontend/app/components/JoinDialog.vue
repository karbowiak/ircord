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
    ><div class="dialog-form pt-0 pr-5 pb-5 pl-5">
      <SettingsDescription>
        Browse {{ chat.network?.name }}, or type a name to join or create one.
      </SettingsDescription>
      <form
        class="join-form flex items-center gap-2"
        @submit.prevent="join(`#${name.replace(/^#/, '')}`)"
      >
        <span class="text-[18px] text-faint">#</span
        ><UiInput
          v-model="name"
          aria-label="Channel name"
          autofocus
          placeholder="Search or enter a channel name"
          maxlength="50"
          required
        /><UiButton :disabled="!name.trim() || chat.join.isLoading">
          Join<AppIcon
            name="right"
            :size="15"
          />
        </UiButton>
      </form>
      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        v-if="error"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <div
        class="join-channel-list mt-3 overflow-y-auto max-h-[min(420px,_55dvh)]"
        :aria-busy="list.isLoading.value"
      >
        <button
          v-if="query && !exact && list.data.value"
          class="join-create flex w-full items-center gap-2.5 rounded-sm p-2 text-left text-muted-foreground hover:bg-accent hover:text-foreground"
          @click="join(`#${query}`)"
        >
          <AppIcon
            name="plus"
            :size="20"
          /><span class="flex min-w-0 flex-1 flex-col"
            ><strong class="text-[14px] text-primary">Create #{{ query }}</strong
            ><small class="overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-faint"
              >Nobody is in it yet; you’ll be its operator.</small
            ></span
          >
        </button>
        <button
          v-for="c in list.data.value?.items"
          :key="c.name"
          class="flex w-full items-center gap-2.5 rounded-sm p-2 text-left text-muted-foreground hover:bg-accent hover:text-foreground"
          @click="join(c.name)"
        >
          <AppIcon
            name="hash"
            :size="20"
          /><span class="flex min-w-0 flex-1 flex-col"
            ><strong class="text-[14px] text-foreground">{{ c.name.replace(/^#/, '') }}</strong
            ><small class="overflow-hidden text-ellipsis whitespace-nowrap text-[12px] text-faint"
              ><MessageContent :text="c.topic || 'No topic'" /></small></span
          ><span
            v-if="joined.has(c.name.toLowerCase())"
            class="joined-badge text-[11px] font-[600] py-[1px] px-1.5 rounded-[4px] bg-selection text-primary"
            >Joined</span
          ><span
            class="channel-member-count flex items-center gap-1 text-[12px] text-faint min-w-22.5 justify-end"
            ><AppIcon
              name="users"
              :size="13"
            />{{ label(c) }}</span
          >
        </button>
        <FieldHint v-if="list.isLoading.value && !list.data.value"> Loading channels… </FieldHint>
        <p
          v-else-if="list.error.value"
          class="form-error text-destructive text-[13px] mt-3"
        >
          {{ errorText(list.error.value) }}
        </p>
        <FieldHint v-else-if="!list.data.value?.items.length && !query">
          This network didn’t list any channels.
        </FieldHint>
      </div>
    </div></AppDialog
  >
</template>
