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
  const bitmap = await createImageBitmap(file)
  try {
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = 256
    const context = canvas.getContext('2d')
    if (!context)
      throw new Error(
        message('mediaUi.avatarImage.prepareFailed', 'Your browser could not prepare this image.'),
      )
    const side = Math.min(bitmap.width, bitmap.height)
    context.drawImage(
      bitmap,
      (bitmap.width - side) / 2,
      (bitmap.height - side) / 2,
      side,
      side,
      0,
      0,
      256,
      256,
    )
    return canvas.toDataURL('image/webp', 0.85)
  } finally {
    bitmap.close()
  }
}
