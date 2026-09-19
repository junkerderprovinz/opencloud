import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ThemeConfigType } from '@opencloud-eu/web-pkg'
import { reloadTheme } from '../../src/reloadTheme'

const initializeThemes = vi.hoisted(() => vi.fn())

vi.mock('@opencloud-eu/web-pkg', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@opencloud-eu/web-pkg')>()),
  useConfigStore: () => ({ theme: '/themes/opencloud/theme.json' }),
  useThemeStore: () => ({ initializeThemes })
}))

const theme: ThemeConfigType = {
  common: { name: 'Knight Cloud', slogan: 'Files, forged' },
  clients: { web: { defaults: { logo: '/themes/_branding/logo-1a2b3c4d5e6f.png' }, themes: [{ isDark: false, label: 'Light' }] } }
}

function serve(status: number) {
  const fetch = vi.fn(async () => ({ ok: status < 300, status, json: async () => theme }))
  vi.stubGlobal('fetch', fetch)
  return fetch
}

describe('reloadTheme', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    initializeThemes.mockReset()
  })

  it('fetches the theme past the browser cache and applies it', async () => {
    const fetch = serve(200)

    await reloadTheme()

    expect(fetch).toHaveBeenCalledWith('/themes/opencloud/theme.json', { cache: 'no-store' })
    expect(initializeThemes).toHaveBeenCalledWith(theme)
  })

  it('fails without applying anything when the theme is not served', async () => {
    serve(503)

    await expect(reloadTheme()).rejects.toThrow('503')
    expect(initializeThemes).not.toHaveBeenCalled()
  })
})
