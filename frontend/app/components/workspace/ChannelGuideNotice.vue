<script setup vapor lang="ts">
/**
 * One-time notice for members who joined before the server had a channel guide (#258). It
 * shows once owners have saved a guide, and never joins anything: opening it shows Featured in
 * Join existing. Dismissing or opening it is remembered with your synced preferences.
 */
const emit = defineEmits<{ open: [] }>()
const { t } = useI18n()
const chat = useChatStore()
const auth = useAuthStore()
const preferences = usePreferencesStore()
const { guide, available } = useChannelGuide(() => chat.networkId)
const shown = computed(
  () =>
    preferences.ready &&
    !preferences.preferences.channelGuideSeen &&
    !auth.account?.onboardingNetworkId &&
    available.value &&
    (guide.data.value?.revision ?? 0) > 0,
)
function seen() {
  preferences.update({ channelGuideSeen: true })
}
function open() {
  seen()
  emit('open')
}
</script>
<template>
  <div
    v-if="shown"
    class="channel-guide-notice relative mt-2 grid gap-1.5 rounded-[var(--radius-large)] bg-card px-3 py-2.5 pr-9 text-[13px] shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]"
    role="region"
    :aria-label="t('channelGuide.notice.title')"
  >
    <strong class="text-foreground">{{ t('channelGuide.notice.title') }}</strong>
    <p class="m-0 text-muted-foreground">
      {{ t('channelGuide.notice.body', { network: chat.network?.name ?? '' }) }}
    </p>
    <UiButton
      size="sm"
      class="w-fit"
      @click="open"
      >{{ t('channelGuide.notice.action') }}</UiButton
    >
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="absolute top-1.5 right-1.5"
      :aria-label="t('channelGuide.notice.dismiss')"
      @click="seen"
      ><AppIcon
        name="close"
        :size="15"
    /></UiButton>
  </div>
</template>
