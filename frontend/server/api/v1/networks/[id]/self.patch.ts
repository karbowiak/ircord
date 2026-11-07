import { getNetwork } from '../../../../utils/demo-store'
import { sendRaw } from '../../../../utils/irc-mock'
import { nicknameError, nicknameLimit } from '../../../../../shared/utils/nickname'

/** The live middleware forwards this contract to Go; fixtures use the same commands. */
export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const id = getRouterParam(event, 'id')!
  const body = await readBody<Record<string, unknown>>(event)
  try {
    const network = getNetwork(session, id)
    if (
      !body ||
      Array.isArray(body) ||
      Object.keys(body).some((key) => !['nick', 'realname', 'away'].includes(key))
    )
      throw new Error('Invalid identity settings.')
    const lines: string[] = []
    for (const [key, value] of Object.entries(body)) {
      if (key === 'nick') {
        const problem =
          typeof value === 'string'
            ? nicknameError(value, nicknameLimit(network.isupport))
            : 'Enter a nickname.'
        if (problem) throw new Error(problem)
        lines.push(`NICK ${value}`)
      } else if (key === 'realname') {
        if (!network.features.setname)
          throw new Error('This network cannot change your real name while connected.')
        if (
          typeof value !== 'string' ||
          !value.trim() ||
          value.length > 128 ||
          /[\r\n\0]/.test(value)
        )
          throw new Error('Enter a valid display name.')
        lines.push(`SETNAME :${value}`)
      } else {
        if (
          value !== null &&
          (typeof value !== 'string' || value.length > 200 || /[\r\n\0]/.test(value))
        )
          throw new Error('Enter a valid away message.')
        lines.push(value ? `AWAY :${value}` : 'AWAY')
      }
    }
    for (const line of lines) sendRaw(session, id, line)
    return network
  } catch (error) {
    routeError(error)
  }
})
