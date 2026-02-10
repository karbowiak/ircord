// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  modules: ['@nuxtjs/tailwindcss', '@pinia/nuxt'],
  runtimeConfig: {
    public: {
      useMockData: process.env.NUXT_PUBLIC_USE_MOCK_DATA ?? 'false',
      ircWebSocketUrl: process.env.NUXT_PUBLIC_IRC_WS_URL ?? 'ws://localhost:8067',
      ircNick: process.env.NUXT_PUBLIC_IRC_NICK ?? 'ircord',
      ircUsername: process.env.NUXT_PUBLIC_IRC_USERNAME ?? 'ircord',
      ircRealname: process.env.NUXT_PUBLIC_IRC_REALNAME ?? 'IRCord User',
      ircServerName: process.env.NUXT_PUBLIC_IRC_SERVER_NAME ?? 'localhost',
    },
  },
  css: ['~/assets/css/main.css'],
  components: [
    { path: '~/components', pathPrefix: false }
  ]
})
