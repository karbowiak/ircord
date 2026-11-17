<template>
  <a
    href="#main"
    class="absolute top-2 left-2 z-10 -translate-y-20 rounded-full bg-accent px-4 py-2 font-semibold text-on-accent focus:translate-y-0"
    >Skip to content</a
  >
  <SiteHeader />
  <main
    id="main"
    tabindex="-1"
    class="outline-none"
  >
    <NuxtPage />
  </main>
  <SiteFooter />
</template>
