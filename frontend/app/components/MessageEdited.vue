<script setup lang="ts">
import type { Message } from '../../shared/types/chat'

/** The "(edited)" marker after an edited message; it toggles the list of earlier versions. */
const props = defineProps<{ message: Message }>()
const open = defineModel<boolean>('open', { default: false })
const { full } = useTimeFormat()
const title = computed(() =>
  props.message.editedAt ? `Edited ${full(props.message.editedAt)}` : 'Edited',
)
</script>
<template>
  <button
    type="button"
    class="message-edited ml-1 text-[0.7em] text-faint align-baseline"
    :title="title"
    :aria-expanded="open"
    :aria-label="`${title}. Show earlier versions`"
    @click.stop="open = !open"
  >
    (edited)
  </button>
</template>
