import { commandAliases, commands, type CommandSpec } from './commands'

export type CommandArgumentValues = Readonly<Record<string, string | undefined>>
export type CommandArgumentError = {
  field: string
  error: 'required' | 'control' | 'token' | 'maxBytes' | 'choice' | 'order'
}

/** Resolve canonical command names and aliases, with an optional leading slash. */
export function resolveCommand(name: string): CommandSpec | undefined {
  const key = name.replace(/^\//, '').toLowerCase()
  return commands.find((command) => command.name === (commandAliases[key] ?? key))
}

/** An exact prefix, before any argument has been typed. */
export function exactParameterizedCommand(input: string): CommandSpec | undefined {
  const match = /^\/([^\s/]+) ?$/u.exec(input)
  const spec = match && resolveCommand(match[1]!)
  return spec && spec.fields.length ? spec : undefined
}

/** Validate individual fields without echoing their values, including secrets. */
export function commandArgumentsValid(
  spec: CommandSpec,
  values: CommandArgumentValues,
): CommandArgumentError | null {
  for (const field of spec.fields) {
    const value = values[field.name] ?? ''
    if (/[\p{Cc}\p{Cs}]/u.test(value)) return { field: field.name, error: 'control' }
    if (field.required && !value.trim()) return { field: field.name, error: 'required' }
    if (!value) continue
    if (field.token && /\s/u.test(value)) return { field: field.name, error: 'token' }
    if (field.maxBytes !== undefined && new TextEncoder().encode(value).length > field.maxBytes)
      return { field: field.name, error: 'maxBytes' }
    if (field.choices && !field.choices.some((choice) => choice.value === value))
      return { field: field.name, error: 'choice' }
  }
  return null
}

/** Serialize positional arguments; domain-specific editors such as status use their own adapter. */
export function serializeCommandArguments(
  spec: CommandSpec,
  values: CommandArgumentValues,
): string {
  if (spec.name === 'status') throw new Error('Status requires its domain adapter')
  const invalid = commandArgumentsValid(spec, values)
  if (invalid) throw new Error(`Invalid command argument: ${invalid.field} (${invalid.error})`)
  const arguments_: string[] = []
  let missing = false
  for (const field of spec.fields) {
    const value = values[field.name] ?? ''
    if (!value) {
      missing = true
    } else {
      if (missing) throw new Error(`Invalid command argument: ${field.name} (order)`)
      arguments_.push(value)
    }
  }
  return `/${spec.name}${arguments_.length ? ` ${arguments_.join(' ')}` : ''}`
}
