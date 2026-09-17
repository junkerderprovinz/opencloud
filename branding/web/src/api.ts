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

export type BrandingHttp = Pick<HttpClient, 'get' | 'put' | 'delete'>

const MB = 1024 * 1024

export const limits: Record<ImageKind, number> = {
  logo: 5 * MB,
  'logo-dark': 5 * MB,
  favicon: 2 * MB,
  background: 25 * MB
}

export const acceptedTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/svg+xml']

export type UploadProblem = 'too-large' | 'unsupported-type'

// Mirrors the server's limits so the admin gets an answer before the upload.
// The server checks again and is the one that decides.
export function validateUpload(kind: ImageKind, file: { size: number; type: string }): UploadProblem | null {
  if (!acceptedTypes.includes(file.type)) {
    return 'unsupported-type'
  }
  if (file.size > limits[kind]) {
    return 'too-large'
  }
  return null
}

const base = 'brandingsvc/api'
const headers = { 'X-Branding-Request': '1' }

export function brandingApi(http: BrandingHttp) {
  return {
    async state() {
      return (await http.get<BrandingState>(`${base}/state`, { headers })).data as BrandingState
    },
    async saveText(name: string, slogan: string) {
      return (await http.put<BrandingState>(`${base}/text`, { name, slogan }, { headers })).data as BrandingState
    },
    async uploadImage(kind: ImageKind, file: Blob) {
      const config = { headers: { ...headers, 'Content-Type': 'application/octet-stream' } }
      return (await http.put<BrandingState>(`${base}/image/${kind}`, file, config)).data as BrandingState
    },
    async clearImage(kind: ImageKind) {
      return (await http.delete<BrandingState>(`${base}/image/${kind}`, undefined, { headers })).data as BrandingState
    }
  }
}
