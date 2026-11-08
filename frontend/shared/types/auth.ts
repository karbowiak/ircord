import type { Account } from './chat'

/**
 * Username and password accounts proposed in #64, ahead of the Go contract. The username is
 * also the account and nick on irc.ircord.com. Accounts from before #64 have no username yet;
 * they still sign in with their key and are asked to choose one.
 */
export type SignedInAccount = Account & { username?: string }

export interface UsernameCheck {
  available: boolean
  /** Why it can't be used: invalid, or already taken on the network. */
  reason?: string
}

export interface Credentials {
  username: string
  password: string
}
