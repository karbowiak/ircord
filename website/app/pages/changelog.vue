<script setup lang="ts">
import { releases } from '../data'
import { site } from '../site'

const description = 'What changed in each IRCord release: new features, improvements and fixes.'
useSeoMeta({
  title: 'What’s new in IRCord',
  description,
  ogTitle: 'What’s new in IRCord',
  ogDescription: description,
  ogUrl: `${site.url}/changelog`,
})
useHead({ link: [{ rel: 'canonical', href: `${site.url}/changelog` }] })
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 pt-16 pb-20 sm:px-6">
    <p class="eyebrow">Changelog</p>
    <h1 class="mt-3 text-[36px] leading-tight font-bold tracking-tight sm:text-[48px]">
      What’s new
    </h1>
    <p class="mt-4 text-[17px] leading-relaxed text-muted">
      Each release lists what is new, improved and fixed for people using IRCord. Earlier releases
      are on
      <a
        :href="site.releases"
        class="prose-link"
        >GitHub</a
      >.
    </p>
    <ol class="mt-12 grid gap-6">
      <li
        v-for="release in releases"
        :key="release.version"
      >
        <article
          :id="release.version"
          :aria-labelledby="`${release.version}-title`"
          class="panel scroll-mt-4 p-6 sm:p-8"
        >
          <h2
            :id="`${release.version}-title`"
            class="flex flex-wrap items-baseline gap-x-3 text-[24px] font-bold tracking-tight"
          >
            {{ release.version }}
            <time
              v-if="release.date"
              :datetime="release.date"
              class="text-[15px] font-medium text-muted"
              >{{ formatDate(release.date) }}</time
            >
          </h2>
          <section
            v-for="group in release.groups"
            :key="group.title"
            class="mt-5"
          >
            <h3
              v-if="group.title"
              class="eyebrow"
            >
              {{ group.title }}
            </h3>
            <template
              v-for="(block, index) in group.blocks"
              :key="index"
            >
              <ul
                v-if="block.type === 'list'"
                class="mt-3 grid list-disc gap-2 pl-5 leading-relaxed marker:text-muted"
              >
                <li
                  v-for="(item, itemIndex) in block.items"
                  :key="itemIndex"
                >
                  <InlineText :content="item" />
                </li>
              </ul>
              <p
                v-else
                class="mt-3 leading-relaxed"
              >
                <InlineText :content="block.content" />
              </p>
            </template>
          </section>
        </article>
      </li>
    </ol>
  </div>
</template>
