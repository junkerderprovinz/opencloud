import { afterEach, describe, expect, it, vi } from 'vitest'
import type { BrandingApi, BrandingState } from '../../src/api'
import { useBranding } from '../../src/useBranding'

const saved: BrandingState = {
  name: 'Knight Cloud',
  slogan: 'Files, forged',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginBackgroundActive: false
}

function fakeApi(): BrandingApi {
  return {
    state: vi.fn(async () => saved),
    saveText: vi.fn(async (name: string, slogan: string) => ({ ...saved, name: name.trim(), slogan: slogan.trim() })),
    uploadImage: vi.fn(async () => ({ ...saved, logo: '/themes/_branding/logo-1a2b3c.png' })),
    clearImage: vi.fn(async () => saved)
  }
}

const reloadThemeOk = async () => {}

describe('useBranding', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('keeps unsaved name and slogan when an image is uploaded or reset', async () => {
    const branding = useBranding(fakeApi(), reloadThemeOk)
    await branding.load()
    branding.name.value = 'Unsaved name'
    branding.slogan.value = 'Unsaved slogan'

    await branding.uploadImage('logo', new Blob(['x']))
    expect(branding.state.value.logo).toBe('/themes/_branding/logo-1a2b3c.png')
    await branding.clearImage('logo')

    expect(branding.name.value).toBe('Unsaved name')
    expect(branding.slogan.value).toBe('Unsaved slogan')
    expect(branding.textChanged.value).toBe(true)
  })

  // Another tab or admin saved 'Knight Cloud' after this tab loaded.
  function staleTab() {
    const api = fakeApi()
    vi.mocked(api.state).mockResolvedValueOnce({ ...saved, name: 'Old name', slogan: 'Old slogan' })
    return useBranding(api, reloadThemeOk)
  }

  it('shows name and slogan saved elsewhere once an image change brings them along', async () => {
    const branding = staleTab()
    await branding.load()

    await branding.uploadImage('logo', new Blob(['x']))

    expect(branding.name.value).toBe('Knight Cloud')
    expect(branding.slogan.value).toBe('Files, forged')
    expect(branding.textChanged.value).toBe(false)
  })

  it('updates only the untouched field when the other one is being edited', async () => {
    const branding = staleTab()
    await branding.load()
    branding.slogan.value = 'Unsaved slogan'

    await branding.clearImage('logo')

    expect(branding.name.value).toBe('Knight Cloud')
    expect(branding.slogan.value).toBe('Unsaved slogan')
  })

  it('shows name and slogan as the server saved them', async () => {
    const branding = useBranding(fakeApi(), reloadThemeOk)
    await branding.load()
    branding.name.value = '  Knight Cloud  '

    await branding.saveText()

    expect(branding.name.value).toBe('Knight Cloud')
    expect(branding.textChanged.value).toBe(false)
  })

  it('reports a saved change as saved when the theme reload fails', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const branding = useBranding(fakeApi(), async () => {
      throw new Error('theme reload failed: 502')
    })
    await branding.load()

    await expect(branding.uploadImage('logo', new Blob(['x']))).resolves.toBe(false)
    expect(branding.state.value.logo).toBe('/themes/_branding/logo-1a2b3c.png')
    expect(branding.busy.value).toBe(false)
  })

  it('confirms the reload after a saved change', async () => {
    const branding = useBranding(fakeApi(), reloadThemeOk)
    await branding.load()

    await expect(branding.clearImage('favicon')).resolves.toBe(true)
  })

  it('passes a failed save on and is ready for the next change', async () => {
    const api = fakeApi()
    const refused = Object.assign(new Error('status 415'), { response: { status: 415 } })
    vi.mocked(api.uploadImage).mockRejectedValueOnce(refused)
    const branding = useBranding(api, reloadThemeOk)
    await branding.load()

    await expect(branding.uploadImage('favicon', new Blob(['x']))).rejects.toBe(refused)
    expect(branding.busy.value).toBe(false)
    expect(branding.state.value).toEqual(saved)
  })
})
