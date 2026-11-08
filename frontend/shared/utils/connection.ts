import type { ConnectionInfo } from '../types/chat'

export const capabilityDescriptions: Record<string, string> = {
  'message-tags': 'Client tags, replies and reactions',
  'echo-message': 'Confirms messages you send',
  'server-time': 'Original server timestamps',
  batch: 'Groups related IRC messages',
  'labeled-response': 'Matches server replies to your requests',
  'draft/chathistory': 'Retrieves earlier conversations',
  'draft/multiline': 'Sends long and multiline messages together',
  'draft/metadata-3': 'Shares profile fields with other clients',
  'draft/message-redaction': 'Message deletion, subject to server permissions',
  'draft/read-marker': 'Synchronizes read position with IRC',
  setname: 'Changes your real name without reconnecting',
  'account-tag': 'Identifies the account behind a message',
  'account-notify': 'Updates when someone logs in or out',
  'away-notify': 'Live away status',
  'extended-join': 'Account and real-name details on join',
  chghost: 'Live username and hostname changes',
  'multi-prefix': 'All channel privileges, rather than only the highest',
  'cap-notify': 'Notifies clients when capabilities change',
  sasl: 'Authenticates an IRC account during connection',
}

export function capabilityRows(
  info: Pick<ConnectionInfo, 'offeredCapabilities' | 'negotiatedCapabilities'>,
) {
  const active = new Set(info.negotiatedCapabilities)
  return [...new Set([...Object.keys(info.offeredCapabilities), ...active])]
    .sort((a, b) => a.localeCompare(b))
    .map((name) => ({
      name,
      value: info.offeredCapabilities[name] ?? '',
      active: active.has(name),
      description: capabilityDescriptions[name] ?? '',
    }))
}
