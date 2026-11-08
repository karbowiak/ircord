import { savedRefs } from '../../utils/demo-store'
export default defineEventHandler(async (event) => savedRefs(await demoSession(event)))
