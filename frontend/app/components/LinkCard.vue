<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { linkMediaReasonKey, publicMediaURL } from '../../shared/utils/link-media'

const props = defineProps<{
  card: components['schemas']['LinkMediaCard']
  url: string
  pending?: boolean
  expired?: boolean
  offline?: boolean
  actions?: boolean
}>()
const { t } = useI18n()
const href = computed(() => publicMediaURL(props.url))
const image = computed(() => publicMediaURL(props.card.image || ''))
const icon = computed(
  () => !props.expired && !props.offline && publicMediaURL(props.card.icon || ''),
)
const imageFailed = ref(false)
const iconFailed = ref(false)
watch(image, () => {
  imageFailed.value = false
})
watch(icon, () => {
  iconFailed.value = false
})
const status = computed(() => {
  if (props.offline) return t('workspaceUi.links.offline')
  if (props.pending) return t('mediaUi.linkPreview.loading')
  if (props.expired) return t('mediaUi.linkPreview.expired')
  return props.card.reason ? t(linkMediaReasonKey(props.card.reason)) : ''
})
</script>
<!--
  A fixed box: the parent's reserved `--embed-height`, or 150px. The text keeps four fixed rows
  (site, two title lines, two description lines, status) whether or not each is present, and a
  page image is a small side thumbnail, so nothing a request returns can change the height.
-->
<template>
  <section
    class="link-card group/media relative overflow-hidden rounded-xl border border-solid border-border bg-secondary text-foreground"
    :style="{ height: 'var(--embed-height, 150px)' }"
  >
    <a
      class="flex h-full min-w-0 gap-3 p-3 no-underline"
      :href="href || undefined"
      target="_blank"
      rel="noopener noreferrer nofollow"
    >
      <span
        v-if="image"
        class="link-card-thumb grid size-24 shrink-0 place-items-center overflow-hidden rounded-lg bg-card text-muted-foreground [&>img]:size-full [&>img]:object-cover"
      >
        <NuxtImg
          v-if="!imageFailed && !expired && !offline"
          :src="image"
          alt=""
          width="96"
          height="96"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="imageFailed = true"
        />
        <AppIcon
          v-else
          name="image"
          :size="22"
        />
      </span>
      <span
        class="link-card-text grid min-w-0 flex-1 grid-rows-[18px_40px_38px_18px] content-start gap-1 pe-18"
      >
        <span
          class="link-card-site flex min-w-0 items-center gap-1.5 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
        >
          <NuxtImg
            v-if="icon && !iconFailed"
            :src="icon"
            alt=""
            width="16"
            height="16"
            referrerpolicy="no-referrer"
            @error="iconFailed = true"
          />
          <span class="truncate">{{ card.site }}</span>
        </span>
        <strong
          class="link-card-title line-clamp-2 text-[length:var(--ui-text)] font-semibold leading-5 wrap-anywhere"
          >{{ card.title || card.site }}</strong
        >
        <span
          class="link-card-description line-clamp-2 text-[length:var(--ui-small)] leading-[19px] text-body wrap-anywhere"
          >{{ card.description }}</span
        >
        <span
          class="truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
          :role="status ? 'status' : undefined"
          >{{ status }}</span
        >
      </span>
    </a>
    <MediaLinkActions
      v-if="actions !== false"
      :url="url"
      always-visible
      class="absolute end-2 top-2"
    />
  </section>
</template>
