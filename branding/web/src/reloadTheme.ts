import { ThemeConfig, useConfigStore, useThemeStore } from '@opencloud-eu/web-pkg'

// OpenCloud loads theme.json once at startup. Re-running the same steps
// updates logo, name and favicon in the open tab after a save.
export async function reloadTheme() {
  const configStore = useConfigStore()
  const themeStore = useThemeStore()
  const response = await fetch(configStore.theme, { cache: 'no-store' })
  if (!response.ok) {
    throw new Error(`theme reload failed: ${response.status}`)
  }
  themeStore.initializeThemes(ThemeConfig.parse(await response.json()))
}
