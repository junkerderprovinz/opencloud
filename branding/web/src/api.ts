import type { HttpClient } from '@opencloud-eu/web-pkg'

export type ImageKind = 'logo' | 'logo-dark' | 'favicon' | 'background'

export interface BrandingState {
  name: string
  slogan: string
  logo: string
  logoDark: string
  favicon: string
  background: string
  loginBackgroundActive: boolean
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

export type Failure = 'forbidden' | 'too-large' | 'unsupported-type' | 'other'

export function failureOf(error: Error & { response?: { status: number } }): Failure {
  if (error instanceof FileTooLargeError) {
    return 'too-large'
  }
  switch (error.response?.status) {
    case 401:
    case 403:
      return 'forbidden'
    case 413:
      return 'too-large'
    case 415:
      return 'unsupported-type'
  }
  return 'other'
}

const base = 'brandingsvc/api'
const headers = { 'X-Branding-Request': '1' }

export function brandingApi(http: BrandingHttp) {
  return {
    async state() {
      return (await http.get<BrandingState>(`${base}/state`, { headers })).data
    },
    async saveText(name: string, slogan: string) {
      return (await http.put<BrandingState>(`${base}/text`, { name, slogan }, { headers })).data
    },
    // The format is left to the server, which reads the content; the browser only guesses from the name.
    async uploadImage(kind: ImageKind, file: Blob) {
      if (file.size > limits[kind]) {
        throw new FileTooLargeError(`${kind} is larger than ${limits[kind]} bytes`)
      }
      const config = { headers: { ...headers, 'Content-Type': 'application/octet-stream' } }
      return (await http.put<BrandingState>(`${base}/image/${kind}`, file, config)).data
    },
    // HttpClient.delete hands its config to axios in a slot axios ignores.
    async clearImage(kind: ImageKind) {
      const config = { method: 'DELETE', url: `${base}/image/${kind}`, headers }
      return (await http.request<BrandingState>(config)).data
    }
  }
}
