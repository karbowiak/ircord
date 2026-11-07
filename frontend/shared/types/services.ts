/**
 * Channel services (ChanServ) shapes proposed in #9. These are frontend-side until the Go
 * contract lands; then they come from the generated OpenAPI types and this file goes away.
 *
 * Errors use HTTP status: 501 the network has no supported ChanServ, 403 not logged in or not
 * allowed, 404 the channel isn't registered, 409 already registered, 422 no such account,
 * 504 services didn't answer. `detail` carries the human-readable reason.
 */

/** Persistent channel roles, as ChanServ AMODE letters: founder, admin, operator, half-op, voice. */
export type ServicesRole = 'q' | 'a' | 'o' | 'h' | 'v'

export interface ChannelRegistration {
  registered: boolean
  founder?: string
  registeredAt?: string
  /** The services account you're logged in as on this network; absent when not logged in. */
  account?: string
}

export interface AccessEntry {
  /** A services account name, not a nick. */
  account: string
  mode: ServicesRole
}

export interface TransferResult {
  /** `pending` until the new owner accepts; `done` when an IRC operator transferred directly. */
  status: 'pending' | 'done'
}
