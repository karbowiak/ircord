<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'
import { linkMediaReasonKey, publicMediaURL } from '../../../shared/utils/link-media'

const props = withDefaults(
  defineProps<{
    card: components['schemas']['LinkMediaCard']
    url: string
    pending?: boolean
    expired?: boolean
    offline?: boolean
    actions?: boolean
  }>(),
  { actions: true },
)
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
    class="link-card group/media relative overflow-hidden rounded-xl border border-solid border-border bg-secondary text-foreground [container-type:size]"
    :style="{ height: 'var(--embed-height, 150px)' }"
  >
    <a
      class="flex h-full min-w-0 gap-3 p-3 no-underline"
      :href="href || undefined"
      target="_blank"
      rel="noopener noreferrer nofollow"
    >
      <span
        class="link-card-thumb grid size-24 shrink-0 grid-cols-1 grid-rows-1 place-items-center overflow-hidden rounded-lg bg-card text-muted-foreground [&>img]:size-full [&>img]:object-cover"
      >
        <NuxtImg
          v-if="image && !imageFailed && !expired && !offline"
          :src="image"
          alt=""
          width="96"
          height="96"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="imageFailed = true"
        />
        <SatoriLinkGraphic
          v-else
          :url="url"
          :site="card.site"
          :title="card.title || card.site"
          :description="card.description"
        />
      </span>
      <span
        class="link-card-text grid min-w-0 flex-1 grid-rows-[18px_40px_38px_18px] content-start gap-1"
      >
        <span
          class="link-card-site flex min-w-0 items-center gap-1.5 pe-18 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
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
          class="link-card-title line-clamp-2 pe-18 text-[length:var(--ui-text)] font-semibold leading-5 wrap-anywhere"
          >{{ card.title || card.site }}</strong
        >
        <span
          class="link-card-description line-clamp-2 text-[length:var(--ui-small)] leading-[19px] text-body wrap-anywhere"
          >{{ card.description }}</span
        >
        <span
          class="link-card-status truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
          :role="status ? 'status' : undefined"
          >{{ status }}</span
        >
      </span>
    </a>
    <MediaLinkActions
      v-if="actions !== false"
      :url="url"
      class="absolute end-2 top-2"
    />
  </section>
</template>
<style scoped>
@container (min-height: 148px) {
  .link-card-status {
    position: absolute;
    inset-inline: 12px;
    bottom: 12px;
  }
}

/* A normal 150px card has a 148px inner box after borders. Only smaller audio slots
   yield optional metadata space to the source, title, status and original-link actions. */
@container (max-height: 147px) {
  .link-card-text {
    grid-template-rows: 18px 20px 18px;
    gap: 2px;
    padding-inline-end: 0;
  }

  .link-card-site,
  .link-card-title {
    padding-inline-end: 72px;
  }

  .link-card-title {
    -webkit-line-clamp: 1;
  }

  .link-card-description {
    display: none;
  }

  .link-card-thumb {
    width: 64px;
    height: 64px;
  }
}
</style>
