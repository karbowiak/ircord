import { publicOrigin, publicPageHead } from '../../shared/page-metadata'

export default defineNuxtPlugin({
  name: 'public-page-metadata',
  dependsOn: ['i18n:plugin'],
  setup() {
    const { $i18n } = useNuxtApp()
    const router = useRouter()
    // Retain the server's pinned/proxy-aware origin across client navigation. Its
    // safe canonical path never contains a conversation identifier or search term.
    const initial = document.querySelector<HTMLMetaElement>('meta[property="og:url"]')?.content
    const origin = publicOrigin(initial || window.location.origin)
    useHead(() =>
      publicPageHead(router.currentRoute.value.path, origin, (key, values) =>
        values ? $i18n.t(key, values) : $i18n.t(key),
      ),
    )
  },
})
