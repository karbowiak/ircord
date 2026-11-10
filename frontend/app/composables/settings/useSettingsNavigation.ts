import type { InjectionKey, Ref } from 'vue'

/**
 * Opens a section or a setting (ids from shared/settings-index.ts) from inside the settings
 * panel, through the same path search results and direct links use.
 */
export const settingsNavigationKey: InjectionKey<(target: string) => void> =
  Symbol('settings-navigation')

export const useSettingsNavigation = () => inject(settingsNavigationKey, () => {})

/**
 * The id of the description of the setting row a control sits in, so switches and choices are
 * described by the row's text without each caller wiring it.
 */
export const settingDescriptionKey: InjectionKey<Ref<string | undefined>> =
  Symbol('setting-description')

export const useSettingDescription = () => inject(settingDescriptionKey, ref(undefined))
