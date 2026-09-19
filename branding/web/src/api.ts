import type { HttpClient } from '@opencloud-eu/web-pkg'

export type ImageKind = 'logo' | 'logo-dark' | 'favicon' | 'background'

export interface BrandingState {
  name: string
  slogan: string
  logo: string
  logoDark: string
  favicon: string
  background: string
}

export type BrandingHttp = Pick<HttpClient, 'get' | 'put' | 'request'>

export type BrandingApi = ReturnType<typeof brandingApi>

export const MB = 1024 * 1024

export const limits: Record<ImageKind, number> = {
  logo: 5 * MB,
  'logo-dark': 5 * MB,
  favicon: 2 * MB,
  background: 25 * MB
}

export const acceptedTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml']

export class FileTooLargeError extends Error {}

export type Failure = 'forbidden' | 'too-large' | 'size-refused' | 'unsupported-type' | 'other'

export function failureOf(error: Error & { response?: { status: number } }): Failure {
  if (error instanceof FileTooLargeError) {
    return 'too-large'
  }
  switch (error.response?.status) {
    case 401:
    case 403:
      return 'forbidden'
    case 413:
      return 'size-refused'
    case 415:
      return 'unsupported-type'
  }
  return 'other'
}

const base = 'brandingsvc/api'
const headers = { 'X-Branding-Request': '1' }

// Without its route the request lands at the web service, which answers any path with 200 and its index.html.
function stateOf({ data }: { data: unknown }) {
  if (typeof data !== 'object' || data === null || typeof (data as BrandingState).name !== 'string') {
    throw new Error('brandingsvc answered without a branding state')
  }
  return data as BrandingState
}

export function brandingApi(http: BrandingHttp) {
  return {
    async state() {
      return stateOf(await http.get<BrandingState>(`${base}/state`, { headers }))
    },
    async saveText(name: string, slogan: string) {
      return stateOf(await http.put<BrandingState>(`${base}/text`, { name, slogan }, { headers }))
    },
    // The format is left to the server, which reads the content; the browser only guesses from the name.
    async uploadImage(kind: ImageKind, file: Blob) {
      if (file.size > limits[kind]) {
        throw new FileTooLargeError(`${kind} is larger than ${limits[kind]} bytes`)
      }
      const config = { headers: { ...headers, 'Content-Type': 'application/octet-stream' } }
      return stateOf(await http.put<BrandingState>(`${base}/image/${kind}`, file, config))
    },
    // HttpClient.delete hands its config to axios in a slot axios ignores.
    async clearImage(kind: ImageKind) {
      const config = { method: 'DELETE', url: `${base}/image/${kind}`, headers }
      return stateOf(await http.request<BrandingState>(config))
    }
  }
}
