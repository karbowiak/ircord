import { appearanceBootScript } from './shared/appearance-boot'
import { compileAppearance, defaultAppearance } from './shared/appearance'
import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2026-11-01',
  ssr: false,
  devtools: { enabled: false },
  modules: ['@pinia/nuxt', '@pinia/colada-nuxt'],
  css: ['~/assets/css/fonts.css', '~/assets/css/tailwind.css'],
  vite: {
    plugins: [tailwindcss()],
    optimizeDeps: {
      // Pre-bundle lazy imports up front so Vite doesn't reload when they're first used.
      include: [
        '@lucide/vue',
        '@tanstack/vue-virtual',
        'highlight.js/lib/core',
        'highlight.js/lib/languages/bash',
        'highlight.js/lib/languages/c',
        'highlight.js/lib/languages/cpp',
        'highlight.js/lib/languages/csharp',
        'highlight.js/lib/languages/css',
        'highlight.js/lib/languages/diff',
        'highlight.js/lib/languages/dockerfile',
        'highlight.js/lib/languages/go',
        'highlight.js/lib/languages/ini',
        'highlight.js/lib/languages/java',
        'highlight.js/lib/languages/javascript',
        'highlight.js/lib/languages/json',
        'highlight.js/lib/languages/kotlin',
        'highlight.js/lib/languages/lua',
        'highlight.js/lib/languages/makefile',
        'highlight.js/lib/languages/markdown',
        'highlight.js/lib/languages/php',
        'highlight.js/lib/languages/python',
        'highlight.js/lib/languages/ruby',
        'highlight.js/lib/languages/rust',
        'highlight.js/lib/languages/scss',
        'highlight.js/lib/languages/shell',
        'highlight.js/lib/languages/sql',
        'highlight.js/lib/languages/swift',
        'highlight.js/lib/languages/typescript',
        'highlight.js/lib/languages/xml',
        'highlight.js/lib/languages/yaml',
        'openapi-fetch',
      ],
    },
  },
  nitro: {
    preset: 'bun',
    experimental: { websocket: true },
    compressPublicAssets: true,
  },
  // Nuxt polls /_nuxt/builds/latest.json for a newer build (default hourly). useAppUpdate also
  // checks on every WebSocket reconnect; this poll covers tabs whose socket never dropped.
  experimental: { checkOutdatedBuildInterval: 5 * 60_000 },
  // Baseline hardening for every response. The Content-Security-Policy is added to the page by
  // server/plugins/security.ts, which hashes the inline scripts it contains.
  routeRules: {
    '/**': {
      headers: {
        'X-Content-Type-Options': 'nosniff',
        'X-Frame-Options': 'DENY',
        'Referrer-Policy': 'strict-origin-when-cross-origin',
        'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=()',
        'Cross-Origin-Opener-Policy': 'same-origin',
      },
    },
    '/sw.js': { headers: { 'Cache-Control': 'no-cache', 'Service-Worker-Allowed': '/' } },
    '/api/v1/link-media/widgets/**': {
      headers: { 'X-Frame-Options': 'SAMEORIGIN', 'Cache-Control': 'private, no-store' },
    },
    '/manifest.webmanifest': { headers: { 'Cache-Control': 'no-cache' } },
    // Brand artwork keeps its URL across releases; revalidate so a changed icon is picked up.
    '/favicon.svg': { headers: { 'Cache-Control': 'no-cache' } },
    '/icons/**': { headers: { 'Cache-Control': 'no-cache' } },
  },
  // The release this build was made from (the image's VERSION build argument), bundled into the
  // client so a running page knows its own version. Empty for development builds.
  appConfig: {
    release: { version: process.env.IRCORD_VERSION ?? '' },
  },
  runtimeConfig: {
    backendUrl: 'http://127.0.0.1:8080',
    siteUrl: '',
    demo: false,
    public: {
      // Go/Huma contract (chat, networks, history, realtime).
      apiBase: '/api/v1',
      // Frontend-owned Nitro routes (appearance, picker library).
      appApiBase: '/api/app',
      websocketUrl: '',
      klipyApiKey: '',
      // Public JSON connection presets; never include passwords here.
      networkPresets: '',
      firstConnectionGuide: true,
    },
  },
  hooks: {
    // highlight.js loads only when a code block needs it; don't prefetch it on every page load.
    'build:manifest'(manifest) {
      for (const [key, chunk] of Object.entries(manifest))
        if (key.includes('node_modules/highlight.js/')) chunk.prefetch = false
    },
  },
  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      title: 'IRCord',
      // viewport-fit=cover lets the layout use safe-area insets (notches, home indicator);
      // resizes-content keeps the composer above the on-screen keyboard.
      viewport:
        'width=device-width, initial-scale=1, viewport-fit=cover, interactive-widget=resizes-content',
      meta: [
        { name: 'description', content: 'Your IRC networks, channels and messages in one place.' },
        { name: 'theme-color', content: '#19191f' },
        // Installed (Home Screen) app on iOS, required there for Web Push.
        { name: 'mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-title', content: 'IRCord' },
        { name: 'apple-mobile-web-app-status-bar-style', content: 'black-translucent' },
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'manifest', href: '/manifest.webmanifest' },
        { rel: 'apple-touch-icon', href: '/icons/apple-touch-icon.png' },
      ],
      // Applies the saved appearance before first paint so there is no theme flash.
      script: [{ innerHTML: appearanceBootScript, tagPosition: 'head' }],
      // Generate fallback variables from the same source as saved themes. The pre-paint
      // stylesheet uses :root:root, so a saved appearance always outranks these defaults.
      style: [
        {
          key: 'appearance-defaults',
          innerHTML: compileAppearance(defaultAppearance, 'dark').css.replace(
            ':root:root',
            ':root',
          ),
        },
      ],
    },
  },
})
