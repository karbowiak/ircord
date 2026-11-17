import tailwindcss from '@tailwindcss/vite'
import { site } from './app/site'

// The public website at ircord.com: static HTML and CSS with no client scripts. `nuxt generate`
// renders every page at build time; the Dockerfile serves the result.
export default defineNuxtConfig({
  compatibilityDate: '2026-11-01',
  devtools: { enabled: false },
  modules: ['@nuxt/eslint'],
  css: ['~/assets/css/main.css'],
  features: {
    // The pages work without JavaScript, so none is sent. Styles go to files the CSP allows.
    noScripts: true,
    inlineStyles: false,
  },
  experimental: {
    payloadExtraction: false,
    // Renders app/error.vue to 404.html, which the web server sends for unknown addresses.
    prerenderErrorPages: true,
  },
  app: {
    head: {
      htmlAttrs: { lang: 'en' },
      meta: [
        { name: 'theme-color', content: '#fcf9fa', media: '(prefers-color-scheme: light)' },
        { name: 'theme-color', content: '#111013', media: '(prefers-color-scheme: dark)' },
        { name: 'color-scheme', content: 'light dark' },
        { property: 'og:site_name', content: 'IRCord' },
        { property: 'og:type', content: 'website' },
        { property: 'og:image', content: `${site.url}/og.png` },
        { property: 'og:image:width', content: '1200' },
        { property: 'og:image:height', content: '630' },
        { property: 'og:image:alt', content: 'IRCord: IRC that stays connected.' },
        { name: 'twitter:card', content: 'summary_large_image' },
        { name: 'twitter:image', content: `${site.url}/og.png` },
      ],
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' },
      ],
    },
  },
  // The changelog and collaborators files live at the repository root.
  vite: {
    plugins: [tailwindcss()],
    server: { fs: { allow: ['..'] } },
  },
  nitro: {
    prerender: {
      routes: ['/', '/changelog'],
      // A client-side fallback page has nothing to show without scripts.
      ignore: ['/200.html'],
      crawlLinks: true,
      failOnError: true,
    },
  },
  eslint: { config: { stylistic: false } },
  typescript: { strict: true },
})
