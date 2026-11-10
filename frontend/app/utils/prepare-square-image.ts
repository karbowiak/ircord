/** Browser-only centered square crop, encoded as WebP. Callers own validation and messages. */
export async function prepareSquareImage(
  file: File,
  size: number,
  contextError?: () => Error,
): Promise<string> {
  const bitmap = await createImageBitmap(file)
  try {
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = size
    const context = canvas.getContext('2d')
    if (!context && contextError) throw contextError()
    const side = Math.min(bitmap.width, bitmap.height)
    context!.drawImage(
      bitmap,
      (bitmap.width - side) / 2,
      (bitmap.height - side) / 2,
      side,
      side,
      0,
      0,
      size,
      size,
    )
    return canvas.toDataURL('image/webp', 0.85)
  } finally {
    bitmap.close()
  }
}
