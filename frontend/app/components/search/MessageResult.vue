<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
/**
 * A message in a list of results (search, saved, inbox), laid out like a timeline message: the
 * author, where and when on the first line, then up to three lines of the message. Attributes and
 * listeners go to the button. `plain` leaves hover and selection to a row around it (saved and
 * inbox rows, which also hold a remove button).
 */
defineProps<{
  time?: { iso: string; label: string; title?: string }
  plain?: boolean
  disabledAction?: boolean
  member?: Member
  author?: string
}>()
const nameStyle = useNameStyle()
</script>
<template>
  <button
    type="button"
    :aria-disabled="disabledAction || undefined"
    class="search-message grid w-full [grid-template-columns:40px_minmax(0,1fr)] gap-x-3.5 rounded-md px-3 py-2.5 text-left transition-colors"
    :class="{
      'hover:not-[.is-selected]:bg-accent/60 [&.is-selected]:bg-selection/70': !plain,
      'cursor-not-allowed opacity-75': disabledAction,
    }"
  >
    <UserAvatar
      :member="member"
      class="mt-0.5 size-10! text-[16px]!"
    />
    <span class="flex min-w-0 flex-col gap-0.5">
      <span
        class="search-message-location flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5 text-[12px] text-muted-foreground"
        ><strong
          v-if="author"
          class="truncate text-[15px] font-semibold leading-[1.375] text-foreground"
          :style="nameStyle(member)"
          >{{ author }}</strong
        ><slot name="location" /><slot name="badges" /><time
          v-if="time"
          class="ml-auto pl-2 text-faint tabular-nums whitespace-nowrap"
          :datetime="time.iso"
          :title="time.title"
          >{{ time.label }}</time
        ></span
      >
      <span
        dir="auto"
        class="result-preview text-[length:var(--ui-text)] leading-normal text-body [display:-webkit-box] [-webkit-line-clamp:3] [-webkit-box-orient:vertical] overflow-hidden wrap-anywhere"
        ><slot
      /></span>
    </span>
  </button>
</template>
