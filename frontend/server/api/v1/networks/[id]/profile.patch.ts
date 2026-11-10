import { updateDemoProfile } from '../../../../utils/demo-profile'

export default defineEventHandler((event) => updateDemoProfile(event, getRouterParam(event, 'id')!))
