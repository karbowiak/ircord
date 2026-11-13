<script setup vapor lang="ts">
import type { LinkCard, LinkSource } from '../../../shared/types/chat'
import { linksClassificationOverrides, linkSourceKey } from '../../queries/links'

defineOptions({ inheritAttrs: false })
const props = defineProps<{ card: LinkCard }>()
const emit = defineEmits<{ jump: [source: LinkSource] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const { t } = useI18n()
const attrs = useAttrs()
const { revealed, reveal } = useSensitiveReveal(
  () =>
    JSON.stringify([
      props.card.identity,
      props.card.url,
      props.card.sources.map((source) => [
        source.networkId,
        source.conversationId,
        source.messageId,
      ]),
    ]),
  () => props.card.sources.map((source) => source.networkId),
)

// A current conversation is authoritative only when it belongs to this source's server.
// Missing classifications are treated as sensitive until the server supplies one.
const blocked = computed(() => {
  if (!props.card.sources.length) return 'unknown'
  let unknown = false
  for (const source of props.card.sources) {
    const conversation = chat.data?.conversations.find(
      (item) => item.id === source.conversationId && item.networkId === source.networkId,
    )
    const current = conversation?.channelSettings?.nsfw
    const key = linkSourceKey(source.networkId, source.conversationId)
    const classification =
      typeof current === 'boolean'
        ? current
        : linksClassificationOverrides.has(key)
          ? linksClassificationOverrides.get(key)
          : source.nsfwKnown === true
            ? source.nsfw
            : undefined
    const optedIn =
      !!source.networkId &&
      !!chat.data?.networks.some((network) => network.id === source.networkId) &&
      preferences.allowsNsfw(source.networkId)
    if (optedIn || classification === false) continue
    if (classification === true) return 'nsfw'
    unknown = true
  }
  return unknown ? 'unknown' : null
})
watch(
  blocked,
  (current, previous) => {
    if (current && current !== previous) revealed.value = false
  },
  { flush: 'sync' },
)
</script>

<template>
  <li
    v-if="blocked && !revealed"
    v-bind="attrs"
    class="catalog-card catalog-card-nsfw relative flex min-w-0 flex-col overflow-hidden rounded-lg border border-solid border-border/60 bg-card [&.is-selected]:border-primary"
  >
    <button
      type="button"
      class="relative flex w-full flex-1 flex-col text-left"
      @click="reveal"
    >
      <div
        class="relative grid aspect-video w-full overflow-hidden bg-secondary place-items-center"
      >
        <div
          aria-hidden="true"
          class="absolute inset-0 flex items-center justify-center gap-5 blur-xl opacity-50"
        >
          <span class="h-24 w-28 rounded-full bg-muted-foreground/30" />
          <span class="h-20 w-32 rounded-full bg-primary/30" />
        </div>
        <span
          class="relative rounded-md bg-card px-3 py-1 text-[length:var(--ui-small)] font-semibold text-foreground"
          >{{
            t(
              blocked === 'nsfw'
                ? 'mediaUi.linkCatalog.nsfw'
                : 'mediaUi.linkCatalog.unknownClassification',
            )
          }}</span
        >
      </div>
      <div class="catalog-card-body flex w-full flex-col gap-2 p-3">
        <strong>{{
          t(
            blocked === 'nsfw'
              ? 'mediaUi.linkCatalog.nsfw'
              : 'mediaUi.linkCatalog.unknownClassification',
          )
        }}</strong>
        <p class="m-0 text-[length:var(--ui-small)] text-muted-foreground">
          {{
            t(
              blocked === 'nsfw'
                ? 'mediaUi.linkCatalog.nsfwHint'
                : 'mediaUi.linkCatalog.unknownClassificationHint',
            )
          }}
        </p>
      </div>
    </button>
  </li>
  <LinkCatalogContent
    v-else
    v-bind="attrs"
    :card="card"
    @jump="emit('jump', $event)"
  />
</template>
