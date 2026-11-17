<script setup lang="ts">
import type { NuxtError } from '#app'
import { site } from './site'

// Built as 404.html, which the web server sends for any address without a page (nginx.conf).
const props = defineProps<{ error: NuxtError }>()
const missing = computed(() => props.error.status === 404)
useSeoMeta({
  title: () => (missing.value ? 'Page not found · IRCord' : 'Something went wrong · IRCord'),
  robots: 'noindex',
})
</script>

<template>
  <SiteHeader />
  <main
    id="main"
    class="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6"
  >
    <p class="eyebrow">Error {{ error.status }}</p>
    <h1 class="mt-3 text-[36px] leading-tight font-bold tracking-tight">
      {{ missing ? 'Page not found' : 'Something went wrong' }}
    </h1>
    <p class="mt-4 text-[17px] leading-relaxed text-muted">
      {{
        missing
          ? 'This address has no page. It may have moved, or the link may be wrong.'
          : 'This page could not be shown.'
      }}
    </p>
    <div class="mt-8 flex flex-wrap justify-center gap-3">
      <NuxtLink
        to="/"
        class="btn-primary"
        >Go to the home page</NuxtLink
      >
      <a
        :href="site.app"
        class="btn-secondary"
        >Open IRCord</a
      >
    </div>
  </main>
  <SiteFooter />
</template>
