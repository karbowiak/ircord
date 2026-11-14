import { storeUpload } from '../../../utils/irc-mock'

/** Multipart upload: `file`, plus optional `width`/`height` measured by the browser for images. */
export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const length = Number(getHeader(event, 'content-length') ?? 0)
  if (length > 9 * 1024 * 1024)
    throw createError({ statusCode: 413, statusMessage: 'Files can be up to 8 MB.' })
  const parts = await readMultipartFormData(event)
  const file = parts?.find((part) => part.name === 'file' && part.filename)
  if (!file) throw createError({ statusCode: 400, statusMessage: 'Choose a file to upload.' })
  // Bun's FormData adds a charset to text parts; validate the media type itself.
  const contentType =
    file.type?.split(';', 1)[0]?.trim().toLowerCase() || 'application/octet-stream'
  const field = (name: string) => Number(parts?.find((part) => part.name === name)?.data.toString())
  try {
    return storeUpload(session, {
      name: file.filename!,
      type: contentType,
      data: new Uint8Array(file.data),
      width: field('width'),
      height: field('height'),
    })
  } catch (error) {
    routeError(error)
  }
})
