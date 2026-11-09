import { defineProvider } from '@nuxt/image/runtime'
import { uploadImageVariant } from '../../shared/utils/upload-image'

/**
 * The existing file route owns authorization, bounded resizing and deletion.
 * External URLs and URLs with existing queries remain byte-for-byte intact.
 */
export default defineProvider({
  getImage: (url, { modifiers = {} }) => ({
    url: uploadImageVariant(url, modifiers.width, modifiers.height),
  }),
})
