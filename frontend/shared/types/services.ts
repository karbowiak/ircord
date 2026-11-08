import type { components } from '../generated/api'

/** The same generated contract used by Go HTTP and the demo adapter. */
export type ChannelRegistration = components['schemas']['ChannelRegistration']
export type AccessEntry = components['schemas']['AccessEntry']
export type ServicesRole = AccessEntry['mode']
export type TransferResult = components['schemas']['TransferResult']
