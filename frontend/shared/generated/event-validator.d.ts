import type { components } from './api'
declare const validate: (value: unknown) => value is components['schemas']['ServerEvent']
export default validate
