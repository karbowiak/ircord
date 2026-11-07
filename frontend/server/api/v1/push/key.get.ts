/** The VAPID public key browsers subscribe with. */
export default defineEventHandler(async (event) => {
  await requireAccount(event)
  return { publicKey: (await vapidKeys()).publicKey }
})
