import type { Language } from 'vue3-gettext'
import { limits, MB, type Failure, type ImageKind } from './api'

export function failureText($gettext: Language['$gettext'], failure: Failure, kind?: ImageKind) {
  switch (failure) {
    case 'forbidden':
      return $gettext('Only admins can change the branding.')
    case 'too-large':
      return $gettext('The file is larger than %{size} MB.', { size: String(limits[kind] / MB) })
    case 'size-refused':
      return $gettext(
        'The server refused the file size. A reverse proxy in front of OpenCloud may have a lower upload limit.'
      )
    case 'unsupported-type':
      return $gettext('Use a PNG, JPEG, GIF or WebP image, or a simpler SVG.')
  }
}
