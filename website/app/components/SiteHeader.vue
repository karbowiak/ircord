<script setup lang="ts">
import { site } from '../site'

const links = [
  { label: 'How it works', to: '/#how-it-works' },
  { label: 'Features', to: '/#features' },
  { label: 'Host it', to: '/#host' },
  { label: 'What’s new', to: '/changelog' },
]
</script>

<template>
  <header class="border-b border-line bg-canvas/90">
    <div
      class="mx-auto grid max-w-6xl grid-cols-[auto_auto] items-center justify-between gap-x-6 gap-y-2 px-4 py-3 sm:px-6 md:grid-cols-[auto_1fr_auto]"
    >
      <NuxtLink
        to="/"
        class="flex items-center gap-2.5 rounded-lg text-[17px] font-bold tracking-tight text-ink no-underline"
      >
        <BrandMark class="size-8 flex-none" />
        IRCord
      </NuxtLink>
      <nav
        aria-label="Main"
        class="col-span-2 row-start-2 md:col-span-1 md:col-start-2 md:row-start-1"
      >
        <ul class="flex flex-wrap gap-x-5 gap-y-1 text-[14px] font-medium md:justify-center">
          <li
            v-for="link in links"
            :key="link.to"
          >
            <NuxtLink
              :to="link.to"
              class="text-muted no-underline hover:text-ink"
              >{{ link.label }}</NuxtLink
            >
          </li>
          <li>
            <a
              :href="site.repo"
              class="text-muted no-underline hover:text-ink"
              >GitHub</a
            >
          </li>
        </ul>
      </nav>
      <a
        :href="site.app"
        class="btn-primary min-h-10 px-4 text-[14px]"
        >Open IRCord</a
      >
    </div>
  </header>
</template>
