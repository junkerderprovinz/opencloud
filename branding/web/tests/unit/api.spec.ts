import { Blob as NodeBlob } from 'node:buffer'
import { describe, expect, it } from 'vitest'
import { HttpClient } from '@opencloud-eu/web-pkg'
import { brandingApi, failureOf, FileTooLargeError, type BrandingState, type ImageKind } from '../../src/api'

const MB = 1024 * 1024

// happy-dom's Blob does not identify as one, so axios would send it as JSON.
const file = (content: string | Uint8Array) => new NodeBlob([content]) as unknown as Blob

const state: BrandingState = {
  name: '',
  slogan: '',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginTheme: ''
}

describe('brandingApi', () => {
  // The real HttpClient, because axios decides which arguments reach the wire.
  function recordingApi() {
    const sent: { method?: string; url?: string; data?: unknown; branding?: unknown; contentType?: unknown }[] = []
    const http = new HttpClient({
      adapter: async (config) => {
        sent.push({
          method: config.method,
          url: config.url,
          data: config.data,
          branding: config.headers['X-Branding-Request'],
          contentType: config.headers['Content-Type']
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
    await api.saveLoginTheme('dark')
    await api.uploadImage('logo', file('x'))
    await api.clearImage('favicon')

    expect(sent.map(({ method, url, branding }) => [method, url, branding])).toEqual([
      ['get', 'brandingsvc/api/state', '1'],
      ['put', 'brandingsvc/api/text', '1'],
      ['put', 'brandingsvc/api/login-theme', '1'],
      ['put', 'brandingsvc/api/image/logo', '1'],
      ['delete', 'brandingsvc/api/image/favicon', '1']
    ])
  })

  it('sends the login card as JSON', async () => {
    const { api, sent } = recordingApi()
    await api.saveLoginTheme('auto')

    expect(JSON.parse(sent[0].data as string)).toEqual({ loginTheme: 'auto' })
  })

  it('sends name and slogan as JSON', async () => {
    const { api, sent } = recordingApi()
    await api.saveText('Knight Cloud', 'Files, forged')

    expect(JSON.parse(sent[0].data as string)).toEqual({ name: 'Knight Cloud', slogan: 'Files, forged' })
  })

  it('uploads the file as the raw body and leaves the format to the server', async () => {
    const { api, sent } = recordingApi()
    const svg = file('<svg xmlns="http://www.w3.org/2000/svg"/>')
    await api.uploadImage('logo-dark', svg)

    expect(svg.type).toBe('')
    expect(sent).toHaveLength(1)
    expect(sent[0]).toMatchObject({
      url: 'brandingsvc/api/image/logo-dark',
      data: svg,
      contentType: 'application/octet-stream'
    })
  })

  it.each<[ImageKind, number]>([
    ['logo', 5],
    ['logo-dark', 5],
    ['favicon', 2],
    ['background', 25]
  ])('sends a %s of %i MB and refuses one byte more without sending it', async (kind, limit) => {
    const { api, sent } = recordingApi()
    await api.uploadImage(kind, file(new Uint8Array(limit * MB)))
    await expect(api.uploadImage(kind, file(new Uint8Array(limit * MB + 1)))).rejects.toBeInstanceOf(
      FileTooLargeError
    )

    expect(sent).toHaveLength(1)
  })

  it('fails every call that is answered with a page instead of a branding state', async () => {
    const api = brandingApi(
      new HttpClient({
        adapter: async (config) => ({
          data: '<!DOCTYPE html><html><head><title>OpenCloud</title></head></html>',
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'text/html' },
          config
        })
      })
    )

    const calls = [
      () => api.state(),
      () => api.saveText('Knight Cloud', 'Files, forged'),
      () => api.saveLoginTheme('dark'),
      () => api.uploadImage('logo', file('x')),
      () => api.clearImage('logo')
    ]
    for (const call of calls) {
      const error = await call().then(() => expect.unreachable(), (error: Error) => error)
      expect(failureOf(error)).toBe('other')
    }
  })
})

describe('failureOf', () => {
  const answered = (status: number) => Object.assign(new Error(`status ${status}`), { response: { status } })

  it('reads 401 and 403 as missing admin rights', () => {
    expect(failureOf(answered(401))).toBe('forbidden')
    expect(failureOf(answered(403))).toBe('forbidden')
  })

  it('names a file refused before the upload as too large', () => {
    expect(failureOf(new FileTooLargeError())).toBe('too-large')
  })

  it('keeps a size the server refuses apart from the app limit', () => {
    expect(failureOf(answered(413))).toBe('size-refused')
  })

  it('names a file the server cannot use as an unsupported type', () => {
    expect(failureOf(answered(415))).toBe('unsupported-type')
  })

  it('does not blame the admin for server errors or a lost connection', () => {
    expect(failureOf(answered(500))).toBe('other')
    expect(failureOf(new Error('Network Error'))).toBe('other')
  })
})
