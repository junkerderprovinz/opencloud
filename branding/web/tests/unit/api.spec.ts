import { describe, expect, it } from 'vitest'
import { HttpClient } from '@opencloud-eu/web-pkg'
import { brandingApi, validateUpload, type BrandingState } from '../../src/api'

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
  // The real HttpClient, because axios decides which arguments reach the wire.
  function recordingApi() {
    const sent: { method?: string; url?: string; data?: unknown; branding?: unknown }[] = []
    const http = new HttpClient({
      adapter: async (config) => {
        sent.push({
          method: config.method,
          url: config.url,
          data: config.data,
          branding: config.headers['X-Branding-Request']
        })
        return { data: state, status: 200, statusText: 'OK', headers: {}, config }
      }
    })
    return { api: brandingApi(http), sent }
  }

  it('sends the branding header on every request', async () => {
    const { api, sent } = recordingApi()
    await api.state()
    await api.saveText('Knight Cloud', 'Files, forged')
    await api.uploadImage('logo', new Blob(['x']))
    await api.clearImage('favicon')

    expect(sent.map(({ method, url, branding }) => [method, url, branding])).toEqual([
      ['get', 'brandingsvc/api/state', '1'],
      ['put', 'brandingsvc/api/text', '1'],
      ['put', 'brandingsvc/api/image/logo', '1'],
      ['delete', 'brandingsvc/api/image/favicon', '1']
    ])
  })

  it('sends name and slogan as JSON', async () => {
    const { api, sent } = recordingApi()
    await api.saveText('Knight Cloud', 'Files, forged')

    expect(JSON.parse(sent[0].data as string)).toEqual({ name: 'Knight Cloud', slogan: 'Files, forged' })
  })
})
