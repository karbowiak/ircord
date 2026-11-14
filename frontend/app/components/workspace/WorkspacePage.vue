<script setup lang="ts" vapor>
import { overlayOpen } from '../../../shared/utils/focus'
const { t: $t } = useI18n()
defineProps<{ title: string; description?: string; meta?: string }>()
const emit = defineEmits<{ close: [] }>()
const { mobileNav } = useWorkspacePane()
const root = ref<HTMLElement | null>(null)
function focus() {
  nextTick(() =>
    root.value?.querySelector<HTMLElement>('[autofocus]')?.focus({ preventScroll: true }),
  )
}
onMounted(focus)
onActivated(focus)
function escape(event: KeyboardEvent) {
  if (event.key !== 'Escape' || event.defaultPrevented || overlayOpen()) return
  event.preventDefault()
  emit('close')
}
</script>
<template>
  <!-- Large-title header: the page name leads, its description sits beneath, and the page's
       own actions share the top row with Close. -->
  <section
    ref="root"
    class="workspace-page flex h-full min-h-0 min-w-0 flex-1 flex-col"
    :aria-label="title"
    @keydown="escape"
  >
    <header
      class="workspace-page-header flex min-w-0 shrink-0 items-start gap-3 px-4 pt-5 pb-4 max-[700px]:gap-1.5 max-[700px]:px-1.5 max-[700px]:pt-2 max-[700px]:pb-3"
    >
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="mobile-menu hidden shrink-0 max-[899px]:inline-flex"
        :aria-label="$t('workspaceUi.sidebar.openConversations')"
        @click="mobileNav = true"
        ><AppIcon
          name="menu"
          :size="18"
      /></UiButton>
      <div class="min-w-0 flex-1">
        <h1
          class="m-0 truncate text-[length:var(--ui-page-title)] leading-[1.1] font-[750] tracking-[-0.025em] max-[700px]:text-[24px]"
          :title="title"
        >
          {{ title }}
        </h1>
        <p
          v-if="description"
          class="m-0 mt-1.5 line-clamp-2 max-w-[72ch] text-[length:var(--ui-text)] text-muted-foreground"
        >
          {{ description }}
        </p>
      </div>
      <div class="flex shrink-0 items-center gap-1.5 pt-0.5">
        <span
          v-if="meta"
          class="whitespace-nowrap rounded-full bg-secondary/45 px-3 py-1 text-[length:var(--ui-caption)] font-[550] text-muted-foreground tabular-nums shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
          >{{ meta }}</span
        >
        <slot name="actions" />
        <AppTooltip :text="$t('workspaceUi.sidebar.returnToConversation')"
          ><UiButton
            variant="ghost"
            size="icon-sm"
            class="rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent"
            :aria-label="$t('workspaceUi.sidebar.returnToConversation')"
            @click="emit('close')"
            ><AppIcon
              name="close"
              :size="17" /></UiButton
        ></AppTooltip>
      </div>
    </header>
    <div class="workspace-page-content flex min-h-0 min-w-0 flex-1 flex-col"><slot /></div>
  </section>
</template>
