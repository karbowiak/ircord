<script setup lang="ts">
import type { Conversation, Network } from '../../shared/types/chat'
import { channelSelectable, MAX_LINK_CHANNELS } from '../../shared/utils/link-catalog'

/**
 * Links → the channel picker: every open channel by network, with a search once there are many.
 * It takes focus when it opens, so its boxes are next for the keyboard; Escape or Done closes it.
 */
const props = defineProps<{ channels: Conversation[]; networks: Network[]; picked: string[] }>()
const chosen = defineModel<string[]>({ required: true })
const emit = defineEmits<{ close: [] }>()
const channelQuery = ref('')
const channelGroups = computed(() => {
  const q = channelQuery.value.trim().toLowerCase().replace(/^#/, '')
  return props.networks
    .map((n) => ({
      network: n,
      channels: props.channels.filter(
        (c) => c.networkId === n.id && (!q || c.name.toLowerCase().includes(q)),
      ),
    }))
    .filter((g) => g.channels.length)
})
const panel = ref<HTMLElement | null>(null)
onMounted(() =>
  panel.value?.querySelector<HTMLElement>('input, [role="checkbox"]:not([disabled])')?.focus(),
)
</script>
<template>
  <div
    ref="panel"
    class="links-channel-picker grid gap-2 mt-2.5 mr-5 mb-0 ml-5 p-2.5 max-h-[min(260px,_40dvh)] overflow-y-auto bg-field border border-solid border-border rounded-md"
    role="group"
    aria-label="Channels to list"
    @keydown.esc.prevent.stop="emit('close')"
  >
    <UiInput
      v-if="channels.length > 8"
      v-model="channelQuery"
      type="search"
      placeholder="Find a channel"
      aria-label="Find a channel"
    />
    <fieldset
      v-for="group in channelGroups"
      :key="group.network.id"
      class="m-0 flex min-w-0 flex-wrap gap-x-3.5 gap-y-1 border-0 p-0"
    >
      <legend
        v-if="networks.length > 1"
        class="mb-1 w-full text-[12px] font-[600] text-muted-foreground"
      >
        {{ group.network.name }}
      </legend>
      <label
        v-for="c in group.channels"
        :key="c.id"
        class="links-channel-option inline-flex items-center gap-1.5 min-h-8 text-[13px] wrap-anywhere"
        ><UiCheckbox
          :model-value="chosen.includes(c.id)"
          :disabled="!channelSelectable(picked, c.id)"
          @update:model-value="
            chosen = $event ? [...chosen, c.id] : chosen.filter((id) => id !== c.id)
          "
        /><span>#{{ c.name }}</span></label
      >
    </fieldset>
    <p
      v-if="!channelGroups.length"
      class="links-channel-none m-0 text-[13px] text-muted-foreground"
    >
      No channel matches.
    </p>
    <p
      v-if="picked.length >= MAX_LINK_CHANNELS"
      class="links-channel-none m-0 text-[13px] text-muted-foreground"
    >
      Up to {{ MAX_LINK_CHANNELS }} channels can be chosen at once.
    </p>
    <div class="links-channel-actions flex justify-end gap-3">
      <UiButton
        variant="link"
        size="text"
        type="button"
        :disabled="!picked.length"
        @click="chosen = []"
      >
        Any channel
      </UiButton>
      <UiButton
        variant="link"
        size="text"
        type="button"
        @click="emit('close')"
      >
        Done
      </UiButton>
    </div>
  </div>
</template>
