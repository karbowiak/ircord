<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { Message } from '../../shared/types/chat'
import { publicMediaURL } from '../../shared/utils/link-media'
import { linkMediaQuery } from '../queries/link-media'
const props = defineProps<{ message: Message; url: string; nearby: boolean }>()
const auth = useAuthStore()
const widgetOnly = ref(false)
const retrying = ref(false)
const busy = computed(() => retrying.value || query.asyncStatus.value === 'loading')
const query = useQuery(() => linkMediaQuery({
  account: auth.account?.id ?? '', conversation: props.message.conversationId,
  message: props.message.id, url: props.url, widget: widgetOnly.value, enabled: props.nearby,
}))
const content = computed(() => query.data.value)
const original = computed(() => publicMediaURL(props.url))
const host = computed(() => { try { return new URL(props.url).hostname.replace(/^www\./, '') } catch { return 'link' } })
const failure = computed(() => errorStatus(query.error.value) === 422
  ? errorText(query.error.value)
  : `Couldn’t display ${host.value}.`)
function fallback() { widgetOnly.value = true }
/** Links whose placeholder takes the shape they will have once resolved. */
const nativeHost = computed(() => /(^|\.)(reddit|instagram|tiktok)\.com$/.test(host.value) && !/^(www\.)?(instagram|tiktok)\.com\/@?[^/]+\/?$/.test(props.url.replace(/^https?:\/\//, '')))
const framedHost = computed(() => /^(x|twitter)\.com$|^bsky\.app$/.test(host.value) && /\/(status|post)\//.test(props.url))
/** A native post with only text (a Reddit self post) is still the post, not a loading link. */
const nativeText = computed(() => ['reddit', 'instagram', 'tiktok'].includes(content.value?.provider ?? '') && !!(content.value?.title || content.value?.description))
async function retry() {
  if(busy.value) return
  retrying.value=true
  try { await Promise.all([query.refetch(),new Promise(resolve=>setTimeout(resolve,300))]) }
  catch { /* The query retains the error for the visible failure state. */ }
  finally {retrying.value=false}
}
</script>
<template>
  <div class="message-link" :aria-busy="busy || undefined">
    <DirectLinkMedia v-if="content?.provider === 'media' && content.assets[0]" :asset="content.assets[0]" :nearby="nearby" @unavailable="fallback" />
    <NativeLinkMedia v-else-if="content && (content.assets.length || nativeText)" :content="content" :nearby="nearby" @unavailable="fallback" />
    <LinkCard v-else-if="content?.card" :card="content.card" :url="url" />
    <LinkWidget v-else-if="content?.widget" :widget="content.widget" :title="content.title || host" :href="original" />
    <div v-else-if="query.error.value" class="link-state link-failed">
      <span>{{ failure }}</span>
      <button type="button" class="link-retry" :disabled="busy" @click="retry"><AppIcon v-if="busy" name="refresh" :size="15" class="retry-spinner" />{{ busy ? 'Retrying…' : 'Retry' }}</button>
      <a :href="original" target="_blank" rel="noopener noreferrer">Open link<AppIcon name="external" :size="12" /></a>
    </div>
    <LinkCard v-else-if="content" :card="{ site: host, title: content.title || host, description: content.description }" :url="url" />
    <section v-else-if="nativeHost" class="native-link-media native-skeleton" role="status">
      <header class="native-media-head" />
      <div class="native-media-copy" />
      <div class="native-media-stage" />
      <span class="sr-only">Loading {{ host }}…</span>
    </section>
    <div v-else-if="framedHost" class="link-widget-skeleton" role="status"><span class="sr-only">Loading {{ host }}…</span></div>
    <div v-else class="link-card link-card-skeleton" role="status">
      <span class="link-card-image" />
      <span class="link-card-text"><span class="link-card-site" /><span class="link-card-title" /><span class="link-card-description" /></span>
      <span class="sr-only">Loading {{ host }}…</span>
    </div>
  </div>
</template>
