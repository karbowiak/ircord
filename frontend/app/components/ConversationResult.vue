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
    class="search-conversation group flex w-full items-center gap-3 rounded-md px-3 py-2 text-left transition-colors hover:not-[.is-selected]:bg-accent/60 [&.is-selected]:bg-selection/70 [&_.mention-badge]:mr-1 [&_.unread-dot]:mr-1"
  >
    <slot name="icon"
      ><span
        class="search-result-icon flex size-9 shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground"
        ><AppIcon
          :name="icon ?? 'hash'"
          :size="18" /></span
    ></slot>
    <span class="flex min-w-0 flex-1 flex-col"
      ><strong class="truncate text-[length:var(--ui-text)] font-semibold">{{ title }}</strong
      ><small
        v-if="detail"
        class="truncate text-[length:var(--ui-caption)] text-muted-foreground"
        >{{ detail }}</small
      ></span
    >
    <slot name="badge" />
    <AppIcon
      :name="action"
      :size="16"
      class="text-muted-foreground opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100 group-[.is-selected]:opacity-100"
    />
  </button>
</template>
