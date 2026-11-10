<script setup vapor lang="ts">
import type { Message } from '../../shared/types/chat'

/** The "(edited)" marker after an edited message; it toggles the list of earlier versions. */
const props = defineProps<{ message: Message }>()
const open = defineModel<boolean>('open', { default: false })
const { full } = useTimeFormat()
const { t } = useI18n()
const $t = t
const title = computed(() =>
  props.message.editedAt
    ? t('messageEdit.editedAt', { time: full(props.message.editedAt) })
    : t('messageEdit.edited'),
)
</script>
<template>
  <button
    type="button"
    class="message-edited ml-1 text-[0.7em] text-faint align-baseline hover:underline focus-visible:underline"
    :title="title"
    :aria-expanded="open"
    :aria-label="$t('messageEdit.showEarlier', { title })"
    @click.stop="open = !open"
  >
    {{ $t('messageEdit.marker') }}
  </button>
</template>
