import { inboxUnread } from '../../../utils/demo-inbox'
export default defineEventHandler(async (event) => ({ unread: inboxUnread(await demoSession(event)) }))
