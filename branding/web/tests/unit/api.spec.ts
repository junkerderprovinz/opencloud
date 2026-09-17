import { describe, expect, it, vi } from 'vitest'
import { brandingApi, validateUpload, type BrandingHttp, type BrandingState } from '../../src/api'

const state: BrandingState = {
  name: '',
  slogan: '',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginBackgroundActive: false
}

describe('validateUpload', () => {
  it('accepts a PNG logo under 5 MB', () => {
    expect(validateUpload('logo', { size: 1024, type: 'image/png' })).toBeNull()
  })
  it('rejects a favicon over 2 MB', () => {
    expect(validateUpload('favicon', { size: 2 * 1024 * 1024 + 1, type: 'image/png' })).toBe('too-large')
  })
  it('allows a 25 MB background', () => {
    expect(validateUpload('background', { size: 25 * 1024 * 1024, type: 'image/jpeg' })).toBeNull()
  })
  it('accepts SVG and rejects other types', () => {
    expect(validateUpload('logo', { size: 10, type: 'image/svg+xml' })).toBeNull()
    expect(validateUpload('logo', { size: 10, type: 'application/pdf' })).toBe('unsupported-type')
  })
})

describe('brandingApi', () => {
  it('sends the branding header on every call', async () => {
    const http = {
      get: vi.fn().mockResolvedValue({ data: state }),
      put: vi.fn().mockResolvedValue({ data: state }),
      delete: vi.fn().mockResolvedValue({ data: state })
    }
    const api = brandingApi(http as unknown as BrandingHttp)
    await api.state()
    await api.saveText('Knight Cloud', 'Files, forged')
    await api.uploadImage('logo', new Blob(['x']))
    await api.clearImage('favicon')

    for (const call of [...http.get.mock.calls, ...http.put.mock.calls, ...http.delete.mock.calls]) {
      expect(call[call.length - 1].headers['X-Branding-Request']).toBe('1')
    }
    expect(http.get).toHaveBeenCalledWith('brandingsvc/api/state', expect.anything())
    expect(http.put).toHaveBeenCalledWith(
      'brandingsvc/api/text',
      { name: 'Knight Cloud', slogan: 'Files, forged' },
      expect.anything()
    )
    expect(http.delete).toHaveBeenCalledWith('brandingsvc/api/image/favicon', undefined, expect.anything())
  })
})
