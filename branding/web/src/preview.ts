import type { WebThemeType } from '@opencloud-eu/web-pkg'
import type { BrandingState, ImageKind } from './api'

export interface Preview {
  url: string
  source: 'custom' | 'logo' | 'default'
  chrome?: { background: string; color: string }
}

// The runtime falls back the same way when a theme has no chrome colours.
function chromeOf(theme?: WebThemeType) {
  const roles = theme?.designTokens?.roles
  const background = roles?.chrome ?? roles?.surfaceContainer
  const color = roles?.onChrome ?? roles?.onSurface
  return background && color ? { background, color } : undefined
}

// A theme that missed the reload after a reset still names the removed image.
function savedOrDefault(saved: string, stock = ''): Preview {
  if (saved) {
    return { url: saved, source: 'custom' }
  }
  return { url: stock.includes('_branding/') ? '' : stock, source: 'default' }
}

export function previewOf(kind: ImageKind, state: BrandingState, themes: WebThemeType[]): Preview {
  const light = themes.find((theme) => !theme.isDark)
  const dark = themes.find((theme) => theme.isDark)
  switch (kind) {
    case 'logo':
      return { ...savedOrDefault(state.logo, light?.logo), chrome: chromeOf(light) }
    case 'logo-dark':
      // brandingd puts the light logo into the dark theme until a dark one is saved.
      if (state.logo && !state.logoDark) {
        return { url: state.logo, source: 'logo', chrome: chromeOf(dark) }
      }
      return { ...savedOrDefault(state.logoDark, dark?.logo), chrome: chromeOf(dark) }
    case 'favicon':
      return savedOrDefault(state.favicon, light?.favicon)
    case 'background':
      return savedOrDefault(state.background)
  }
}
