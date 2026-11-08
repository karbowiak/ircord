<script setup lang="ts">
import type { LinkCard, LinkKind, LinkSource } from '../../shared/types/chat'
import { linkKinds, linkTitle, openableLink } from '../../shared/utils/link-catalog'

/**
 * One link in the Links dialog: what it points to, its latest post, actions, and every post of it
 * on request. Choosing a post emits `jump`; the dialog checks the channel is still open.
 */
defineProps<{ card: LinkCard }>()
const emit = defineEmits<{ jump: [source: LinkSource] }>()
const chat = useChatStore()
const { full, day, time } = useTimeFormat()
const when = (iso: string) => `${day(iso)} ${time(iso)}`

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
const networkName = (id: string) => chat.data?.networks.find((n) => n.id === id)?.name ?? 'Network'
const place = (s: LinkSource) => {
  const c = conversation(s.conversationId)
  return c ? `#${c.name}` : 'a closed channel'
}
const author = (s: LinkSource) =>
  chat.member(s.senderId)?.displayName || chat.member(s.senderId)?.nick || 'Someone'
const kindIcon = (k: LinkKind) => linkKinds.find((x) => x.value === k)?.icon ?? 'link'
const expanded = ref(false)

async function copy(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    chat.notify('Link copied.')
  } catch {
    chat.notify('Couldn’t copy the link. Your browser blocked clipboard access.')
  }
}
</script>
<template>
  <li class="catalog-card p-2 rounded-sm hover:bg-accent [&.is-selected]:bg-accent">
    <div class="catalog-card-main flex items-start gap-3 min-w-0 max-[600px]:flex-wrap">
      <span
        class="catalog-card-thumb flex-none grid [place-items:center] w-16 h-16 overflow-hidden rounded-sm bg-accent text-muted-foreground max-[600px]:size-12"
        aria-hidden="true"
        ><AppIcon
          :name="kindIcon(card.kind)"
          :size="22"
      /></span>
      <div class="catalog-card-text flex-1 min-w-0 flex flex-col gap-0.5">
        <a
          v-if="openableLink(card.url)"
          class="catalog-card-title text-[14px] font-[600] text-foreground wrap-anywhere [display:-webkit-box] [-webkit-line-clamp:2] [-webkit-box-orient:vertical] overflow-hidden"
          :href="card.url"
          target="_blank"
          rel="noopener noreferrer nofollow"
          >{{ linkTitle(card) }}</a
        ><span
          v-else
          class="catalog-card-title text-[14px] font-[600] text-foreground wrap-anywhere [display:-webkit-box] [-webkit-line-clamp:2] [-webkit-box-orient:vertical] overflow-hidden"
          >{{ linkTitle(card) }}</span
        >
        <span class="catalog-card-host text-[12px] text-muted-foreground">{{ card.host }}</span>
        <span class="catalog-card-source text-[12px] text-muted-foreground"
          ><strong>{{ author(card.sources[0]!) }}</strong> in {{ place(card.sources[0]!) }} ·
          {{ networkName(card.sources[0]!.networkId) }} ·
          <time
            class="text-faint"
            :datetime="card.lastPostedAt"
            :title="full(card.lastPostedAt)"
            >{{ when(card.lastPostedAt) }}</time
          ></span
        >
        <span
          v-if="card.sources[0]?.excerpt"
          class="catalog-card-excerpt overflow-hidden text-[13px] text-body wrap-anywhere [-webkit-box-orient:vertical] [-webkit-line-clamp:2] [display:-webkit-box]"
          >{{ card.sources[0].excerpt }}</span
        >
      </div>
      <div
        class="catalog-card-actions flex-none flex gap-0.5 max-[600px]:w-full max-[600px]:justify-end"
      >
        <UiButton
          variant="ghost"
          size="icon-sm"
          :aria-label="`Jump to the message: ${linkTitle(card)}`"
          title="Jump to message"
          @click="emit('jump', card.sources[0]!)"
        >
          <AppIcon
            name="reply"
            :size="17"
          />
        </UiButton>
        <UiButton
          variant="ghost"
          size="icon-sm"
          :aria-label="`Copy link: ${card.url}`"
          title="Copy link"
          @click="copy(card.url)"
        >
          <AppIcon
            name="copy"
            :size="17"
          />
        </UiButton>
        <UiButton
          as="a"
          variant="ghost"
          size="icon-sm"
          v-if="openableLink(card.url)"
          :href="card.url"
          target="_blank"
          rel="noopener noreferrer nofollow"
          :aria-label="`Open link: ${card.url}`"
          title="Open link"
          ><AppIcon
            name="external"
            :size="17"
        /></UiButton>
      </div>
    </div>
    <UiButton
      variant="link"
      size="text"
      v-if="card.count > 1"
      class="catalog-card-count mt-1 mr-0 mb-0 ml-19 text-[12px] max-[600px]:ml-0"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      Posted {{ card.count }} times{{ expanded ? '' : ' · show posts' }}
    </UiButton>
    <ul
      v-if="expanded"
      class="catalog-card-sources list-none mt-1 mr-0 mb-0 ml-0 pt-0 pr-0 pb-0 pl-17 max-[600px]:pl-0"
      :aria-label="`Posts of ${linkTitle(card)}`"
    >
      <li
        v-for="source in card.sources"
        :key="source.messageId"
      >
        <MessageResult
          :time="{
            iso: source.postedAt,
            label: full(source.postedAt),
            title: full(source.postedAt),
          }"
          @click="emit('jump', source)"
        >
          <template #location>{{ networkName(source.networkId) }} · {{ place(source) }}</template
          ><span
            ><strong>{{ author(source) }}</strong> {{ source.excerpt }}</span
          >
        </MessageResult>
      </li>
      <li
        v-if="card.count > card.sources.length"
        class="catalog-card-more py-1 px-2 text-[12px] text-faint"
      >
        and {{ card.count - card.sources.length }} earlier
        {{ card.count - card.sources.length === 1 ? 'post' : 'posts' }}
      </li>
    </ul>
  </li>
</template>
