<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { Message } from '../../shared/types/chat'
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
import { linkMediaQuery } from '../queries/link-media'
import { offlineStatus } from '../utils/offline-archive'
const props = defineProps<{ message: Message; url: string; nearby: boolean }>()
const { t } = useI18n()
const auth = useAuthStore()
const widgetOnly = ref(false)
const retrying = ref(false)
const saved = useOfflineMedia(() => props.url)
const offlineAsset = computed<components['schemas']['LinkMediaAsset'] | undefined>(() => {
  const kind = saved.contentType.value.split('/')[0]
  return saved.saved.value && (kind === 'image' || kind === 'video' || kind === 'audio')
    ? { url: props.url, kind, contentType: saved.contentType.value }
    : undefined
})
const busy = computed(() => retrying.value || query.asyncStatus.value === 'loading')
const query = useQuery(() =>
  linkMediaQuery({
    account: auth.account?.id ?? '',
    conversation: props.message.conversationId,
    message: props.message.id,
    url: props.url,
    widget: widgetOnly.value,
    enabled: props.nearby && !offlineStatus.value.offline,
  }),
)
const content = computed(() => query.data.value)
const original = computed(() => publicMediaURL(props.url))
const host = computed(() => {
  try {
    return new URL(props.url).hostname.replace(/^www\./, '')
  } catch {
    return t('workspaceUi.links.link')
  }
})
const failure = computed(() =>
  errorStatus(query.error.value) === 422
    ? errorText(query.error.value, t)
    : t('workspaceUi.links.failed', { host: host.value }),
)
function fallback() {
  widgetOnly.value = true
}
/** Links whose placeholder takes the shape they will have once resolved. */
const nativeHost = computed(
  () =>
    /(^|\.)(reddit|instagram|tiktok)\.com$/.test(host.value) &&
    !/^(www\.)?(instagram|tiktok)\.com\/@?[^/]+\/?$/.test(props.url.replace(/^https?:\/\//, '')),
)
const framedHost = computed(
  () => /^(x|twitter)\.com$|^bsky\.app$/.test(host.value) && /\/(status|post)\//.test(props.url),
)
/** A native post with only text (a Reddit self post) is still the post, not a loading link. */
const nativeText = computed(
  () =>
    ['reddit', 'instagram', 'tiktok'].includes(content.value?.provider ?? '') &&
    !!(content.value?.title || content.value?.description),
)
async function retry() {
  if (busy.value) return
  retrying.value = true
  try {
    await Promise.all([query.refetch(), new Promise((resolve) => setTimeout(resolve, 300))])
  } catch {
    /* The query retains the error for the visible failure state. */
  } finally {
    retrying.value = false
  }
}
</script>
<template>
  <div
    class="message-link w-[min(100%,_480px)] min-w-0 mt-2"
    :aria-busy="busy || undefined"
  >
    <DirectLinkMedia
      v-if="offlineStatus.offline && offlineAsset"
      :asset="offlineAsset"
      :nearby="nearby"
    />
    <p
      v-else-if="offlineStatus.offline"
      class="link-state text-muted-foreground text-[length:var(--ui-caption)]"
    >
      {{ $t('workspaceUi.links.offline') }}
    </p>
    <DirectLinkMedia
      v-else-if="content?.provider === 'media' && content.assets[0]"
      :asset="content.assets[0]"
      :nearby="nearby"
      @unavailable="fallback"
    />
    <NativeLinkMedia
      v-else-if="content && (content.assets.length || nativeText)"
      :content="content"
      :nearby="nearby"
      @unavailable="fallback"
    />
    <LinkCard
      v-else-if="content?.card"
      :card="content.card"
      :url="url"
    />
    <LinkWidget
      v-else-if="content?.widget"
      :widget="content.widget"
      :title="content.title || host"
      :href="original"
    />
    <div
      v-else-if="query.error.value"
      class="link-state flex flex-wrap items-center gap-2 text-muted-foreground text-[12px] min-h-7 link-failed"
    >
      <span>{{ failure }}</span>
      <button
        type="button"
        class="link-retry inline-flex items-center justify-center gap-1.5 min-h-9 py-1.5 px-3 bg-secondary border border-solid border-border rounded-sm"
        :disabled="busy"
        @click="retry"
      >
        <AppIcon
          v-if="busy"
          name="refresh"
          :size="15"
          class="retry-spinner [animation:link-retry-spin_1s_linear_infinite]"
        />{{ busy ? $t('workspaceUi.links.retrying') : $t('workspaceUi.message.retry') }}
      </button>
      <a
        :href="original"
        target="_blank"
        rel="noopener noreferrer"
        >{{ $t('workspaceUi.links.open')
        }}<AppIcon
          name="external"
          :size="12"
      /></a>
    </div>
    <LinkCard
      v-else-if="content"
      :card="{ site: host, title: content.title || host, description: content.description }"
      :url="url"
    />
    <section
      v-else-if="nativeHost"
      class="native-link-media relative grid overflow-hidden min-w-0 w-[min(100%,_420px)] border border-solid border-border rounded-[14px] bg-secondary [container-type:inline-size] native-skeleton"
      role="status"
    >
      <header
        class="native-media-head flex items-center gap-2 min-w-0 min-h-10 pt-2 pr-2 pb-0 pl-3 text-[12px]"
      />
      <div class="native-media-copy relative block h-19 pt-1.5 pr-3 pb-2.5 pl-3 overflow-hidden" />
      <div
        class="native-media-stage relative w-full min-w-0 h-[var(--stage-max)] bg-[#0b0d12] overflow-hidden"
      />
      <span class="sr-only">{{ $t('workspaceUi.links.loading', { host }) }}</span>
    </section>
    <div
      v-else-if="framedHost"
      class="link-widget-skeleton h-105 rounded-[12px] [background:linear-gradient(100deg,_var(--panel)_30%,_var(--raised)_50%,_var(--panel)_70%)_0_0_/_300%_100%] [animation:link-card-wait_1.4s_linear_infinite]"
      role="status"
    >
      <span class="sr-only">{{ $t('workspaceUi.links.loading', { host }) }}</span>
    </div>
    <div
      v-else
      class="link-card block overflow-hidden text-foreground bg-secondary border border-solid border-border rounded-[12px] no-underline link-card-skeleton"
      role="status"
    >
      <span
        class="link-card-image block [aspect-ratio:1.91] bg-card border-b [border-bottom-style:solid] border-b-border"
      />
      <span class="link-card-text grid gap-1 pt-2.5 pr-3 pb-3 pl-3"
        ><span
          class="link-card-site flex items-center gap-1.5 h-4 overflow-hidden text-muted-foreground text-[12px] leading-[16px] whitespace-nowrap text-ellipsis" /><span
          class="link-card-title [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] overflow-hidden wrap-anywhere h-10 text-[14px] font-[600] leading-[20px]" /><span
          class="link-card-description [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] overflow-hidden wrap-anywhere h-9.5 text-body text-[13px] leading-[19px]"
      /></span>
      <span class="sr-only">{{ $t('workspaceUi.links.loading', { host }) }}</span>
    </div>
  </div>
</template>
