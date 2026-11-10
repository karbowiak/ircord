import { prepareSquareImage } from './prepare-square-image'

/**
 * Turns a chosen PNG, JPEG or WebP file into a 256×256 centre-cropped WebP data URL for the
 * profile image upload. Throws a message fit to show the user.
 */
export type AvatarImageTranslate = (key: string, values?: Record<string, string | number>) => string

export async function prepareAvatarImage(
  file: File,
  translate?: AvatarImageTranslate,
  locale = 'en',
): Promise<string> {
  const message = (key: string, fallback: string, values?: Record<string, string | number>) =>
    translate?.(key, values) ?? fallback
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type))
    throw new Error(
      message('mediaUi.avatarImage.chooseFormat', 'Choose a PNG, JPEG or WebP image.'),
    )
  if (file.size > 5 * 1024 * 1024) {
    const size = `${new Intl.NumberFormat(locale).format(5)} MB`
    throw new Error(
      message('mediaUi.avatarImage.tooLarge', 'Choose an image smaller than 5 MB.', { size }),
    )
  }
  return prepareSquareImage(
    file,
    256,
    () =>
      new Error(
        message('mediaUi.avatarImage.prepareFailed', 'Your browser could not prepare this image.'),
      ),
  )
}
