import { useQuery } from '@pinia/colada'
import { DEFAULT_AVATAR_MANIFEST, parseAvatarManifest } from '../../../shared/utils/default-avatars'

/**
 * The default avatars this client ships, read from their manifest at runtime so new entries
 * appear without a code change. A failed load just leaves the picker empty; uploads still work.
 */
export function useDefaultAvatars() {
  const query = useQuery({
    key: ['default-avatars'],
    query: async () => {
      const response = await fetch(DEFAULT_AVATAR_MANIFEST, { cache: 'no-cache' })
      if (!response.ok) throw new Error(`Default avatars unavailable (${response.status})`)
      return parseAvatarManifest(await response.json())
    },
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  })
  return {
    avatars: computed(() => query.data.value ?? []),
    loading: computed(() => query.isLoading.value),
  }
}
