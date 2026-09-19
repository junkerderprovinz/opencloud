import { describe, expect, it } from 'vitest'
import type { WebThemeType } from '@opencloud-eu/web-pkg'
import type { BrandingState } from '../../src/api'
import { previewOf } from '../../src/preview'

const nothingSaved: BrandingState = {
  name: '',
  slogan: '',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginBackgroundActive: false
}

// The two themes of OpenCloud 7.2, after the runtime merged the defaults into them.
const themes: WebThemeType[] = [
  {
    label: 'Light Theme',
    isDark: false,
    logo: 'themes/opencloud/assets/logo.svg',
    favicon: 'themes/opencloud/assets/favicon.svg',
    designTokens: {
      roles: { chrome: '#20434f', onChrome: '#ffffff', surfaceContainer: '#f6f8fa', onSurface: '#191c1d' }
    }
  },
  {
    label: 'Dark Theme',
    isDark: true,
    logo: 'themes/opencloud/assets/logo-white.svg',
    favicon: 'themes/opencloud/assets/favicon.svg',
    designTokens: { roles: { surfaceContainer: '#1d2021', onSurface: '#e1e3e4' } }
  }
]

const logo = '/themes/_branding/logo-1a2b3c4d5e6f.png'
const logoDark = '/themes/_branding/logo-dark-6f5e4d3c2b1a.svg'

describe('previewOf', () => {
  it('shows the light logo in the dark slot until a dark logo is saved', () => {
    expect(previewOf('logo-dark', { ...nothingSaved, logo }, themes)).toMatchObject({ url: logo, source: 'logo' })
    expect(previewOf('logo-dark', { ...nothingSaved, logo, logoDark }, themes)).toMatchObject({
      url: logoDark,
      source: 'custom'
    })
  })

  it('shows the images OpenCloud uses when nothing is saved', () => {
    expect(previewOf('logo', nothingSaved, themes)).toMatchObject({
      url: 'themes/opencloud/assets/logo.svg',
      source: 'default'
    })
    expect(previewOf('logo-dark', nothingSaved, themes)).toMatchObject({
      url: 'themes/opencloud/assets/logo-white.svg',
      source: 'default'
    })
    expect(previewOf('favicon', nothingSaved, themes)).toMatchObject({
      url: 'themes/opencloud/assets/favicon.svg',
      source: 'default'
    })
    expect(previewOf('background', nothingSaved, themes)).toMatchObject({ url: '', source: 'default' })
  })

  it('trusts the saved state over a theme that missed the last reload', () => {
    const stale = themes.map((theme) => ({ ...theme, logo: 'themes/_branding/logo-0a0b0c0d0e0f.png' }))

    expect(previewOf('logo', nothingSaved, stale)).toMatchObject({ url: '', source: 'default' })
    expect(previewOf('logo', { ...nothingSaved, logo }, stale)).toMatchObject({ url: logo, source: 'custom' })
  })

  it('previews each logo on the top bar colours of its own theme', () => {
    expect(previewOf('logo', nothingSaved, themes).chrome).toEqual({ background: '#20434f', color: '#ffffff' })
    expect(previewOf('logo-dark', { ...nothingSaved, logo }, themes).chrome).toEqual({
      background: '#1d2021',
      color: '#e1e3e4'
    })
    expect(previewOf('favicon', nothingSaved, themes).chrome).toBeUndefined()
  })

  it('leaves the colours to the preview box when the theme names none', () => {
    const plain = themes.map(({ designTokens, ...theme }) => theme)

    expect(previewOf('logo', nothingSaved, plain).chrome).toBeUndefined()
    expect(previewOf('logo-dark', nothingSaved, []).chrome).toBeUndefined()
    expect(previewOf('logo-dark', nothingSaved, []).url).toBe('')
  })
})
