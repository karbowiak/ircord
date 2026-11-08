import { appearanceBootScript } from './shared/appearance-boot'

export default defineNuxtConfig({
  compatibilityDate: '2026-11-01',
  ssr: false,
  devtools: { enabled: false },
  modules: ['@pinia/nuxt', '@pinia/colada-nuxt'],
  css: ['~/assets/css/fonts.css', '~/assets/css/main.css'],
  nitro: { experimental: { websocket: true }, compressPublicAssets: true },
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
    '/manifest.webmanifest': { headers: { 'Cache-Control': 'no-cache' } },
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
    },
  },
})
