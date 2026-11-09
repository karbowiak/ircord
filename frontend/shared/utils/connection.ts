import type { ConnectionInfo } from '../types/chat'

export type CapabilityTranslator = (key: string) => string

const capabilityDescriptionKeys: Record<string, string> = {
  'message-tags': 'messageTags',
  'echo-message': 'echoMessage',
  'server-time': 'serverTime',
  batch: 'batch',
  'labeled-response': 'labeledResponse',
  'draft/chathistory': 'chatHistory',
  'draft/multiline': 'multiline',
  'draft/metadata-3': 'metadata',
  'draft/message-redaction': 'redaction',
  'draft/read-marker': 'readMarker',
  setname: 'setname',
  'account-tag': 'accountTag',
  'account-notify': 'accountNotify',
  'away-notify': 'awayNotify',
  'extended-join': 'extendedJoin',
  chghost: 'chghost',
  'multi-prefix': 'multiPrefix',
  'cap-notify': 'capNotify',
  sasl: 'sasl',
}

const english: CapabilityTranslator = (key) => {
  const messages: Record<string, string> = {
    'collectionsUi.serverCapabilities.messageTags': 'Client tags, replies and reactions',
    'collectionsUi.serverCapabilities.echoMessage': 'Confirms messages you send',
    'collectionsUi.serverCapabilities.serverTime': 'Original server timestamps',
    'collectionsUi.serverCapabilities.batch': 'Groups related IRC messages',
    'collectionsUi.serverCapabilities.labeledResponse': 'Matches server replies to your requests',
    'collectionsUi.serverCapabilities.chatHistory': 'Retrieves earlier conversations',
    'collectionsUi.serverCapabilities.multiline': 'Sends long and multiline messages together',
    'collectionsUi.serverCapabilities.metadata': 'Shares profile fields with other clients',
    'collectionsUi.serverCapabilities.redaction': 'Message deletion, subject to server permissions',
    'collectionsUi.serverCapabilities.readMarker': 'Synchronizes read position with IRC',
    'collectionsUi.serverCapabilities.setname': 'Changes your real name without reconnecting',
    'collectionsUi.serverCapabilities.accountTag': 'Identifies the account behind a message',
    'collectionsUi.serverCapabilities.accountNotify': 'Updates when someone logs in or out',
    'collectionsUi.serverCapabilities.awayNotify': 'Live away status',
    'collectionsUi.serverCapabilities.extendedJoin': 'Account and real-name details on join',
    'collectionsUi.serverCapabilities.chghost': 'Live username and hostname changes',
    'collectionsUi.serverCapabilities.multiPrefix':
      'All channel privileges, rather than only the highest',
    'collectionsUi.serverCapabilities.capNotify': 'Notifies clients when capabilities change',
    'collectionsUi.serverCapabilities.sasl': 'Authenticates an IRC account during connection',
  }
  return messages[key] ?? key
}

export function capabilityRows(
  info: Pick<ConnectionInfo, 'offeredCapabilities' | 'negotiatedCapabilities'>,
  translate: CapabilityTranslator = english,
) {
  const active = new Set(info.negotiatedCapabilities)
  return [...new Set([...Object.keys(info.offeredCapabilities), ...active])]
    .sort((a, b) => a.localeCompare(b))
    .map((name) => ({
      name,
      value: info.offeredCapabilities[name] ?? '',
      active: active.has(name),
      description: capabilityDescriptionKeys[name]
        ? translate(`collectionsUi.serverCapabilities.${capabilityDescriptionKeys[name]}`)
        : '',
    }))
}
