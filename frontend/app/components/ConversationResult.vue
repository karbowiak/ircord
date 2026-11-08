<script setup lang="ts">
/**
 * A conversation, server window or person in search results: an icon (or the `icon` slot), a
 * name and a detail line, then the `badge` slot and an action hint shown on hover. Attributes and
 * listeners go to the button.
 */
withDefaults(defineProps<{ title: string; detail?: string; icon?: string; action?: string }>(), {
  detail: undefined,
  icon: undefined,
  action: 'enter',
})
</script>
<template>
  <button
    type="button"
    class="search-conversation group flex w-full items-center gap-2.5 rounded-sm px-2 py-1.5 text-left hover:not-[.is-selected]:bg-accent [&.is-selected]:bg-selection [&_.mention-badge]:mr-1 [&_.unread-dot]:mr-1"
  >
    <slot name="icon"
      ><span
        class="search-result-icon flex h-9 w-9 items-center justify-center rounded-full bg-secondary text-muted-foreground"
        ><AppIcon :name="icon ?? 'hash'" /></span
    ></slot>
    <span class="flex min-w-0 flex-1 flex-col"
      ><strong class="text-[14px] font-[550]">{{ title }}</strong
      ><small
        v-if="detail"
        class="text-[12px] text-faint"
        >{{ detail }}</small
      ></span
    >
    <slot name="badge" />
    <AppIcon
      :name="action"
      :size="16"
      class="text-faint opacity-0 group-hover:opacity-100"
    />
  </button>
</template>
