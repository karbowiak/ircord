import { publicOrigin, publicPageHead } from '../../shared/page-metadata'

export default defineNuxtPlugin(() => {
  const router = useRouter()
  // Retain the server's pinned/proxy-aware origin across client navigation. Its
  // safe canonical path never contains a conversation identifier or search term.
  const initial = document.querySelector<HTMLMetaElement>('meta[property="og:url"]')?.content
  const origin = publicOrigin(initial || window.location.origin)
  useHead(() => publicPageHead(router.currentRoute.value.path, origin))
})
