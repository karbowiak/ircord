<script setup lang="ts">
import { site } from '../site'
</script>

<template>
  <footer class="border-t border-line">
    <div
      class="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-8 text-[14px] text-muted sm:flex-row sm:items-center sm:justify-between sm:px-6"
    >
      <p class="flex items-center gap-2">
        <BrandMark class="size-5 flex-none" />
        IRCord is open source under the MIT license.
      </p>
      <nav aria-label="Footer">
        <ul class="flex flex-wrap gap-x-5 gap-y-1">
          <li><a :href="site.repo">GitHub</a></li>
          <li><NuxtLink to="/changelog">What’s new</NuxtLink></li>
          <li><a :href="site.issues">Report a problem</a></li>
          <li><a :href="site.app">chat.ircord.com</a></li>
        </ul>
      </nav>
    </div>
  </footer>
</template>

<style scoped>
a {
  color: var(--muted);
  text-decoration: none;
}
a:hover {
  color: var(--ink);
  text-decoration: underline;
}
</style>
