import { describe, expect, it } from 'vitest'
import { createGettext } from 'vue3-gettext'
import { failureText } from '../../src/feedback'

const { $gettext } = createGettext({ translations: {}, silent: true })

describe('failureText', () => {
  it('names the limit of the image that was too large', () => {
    expect(failureText($gettext, 'too-large', 'favicon')).toBe('The file is larger than 2 MB.')
    expect(failureText($gettext, 'too-large', 'background')).toBe('The file is larger than 25 MB.')
  })

  it('points at a proxy when the server refuses a size within the limit', () => {
    expect(failureText($gettext, 'size-refused', 'background')).toBe(
      'The server refused the file size. A reverse proxy in front of OpenCloud may have a lower upload limit.'
    )
  })

  it('explains missing admin rights and refused formats', () => {
    expect(failureText($gettext, 'forbidden')).toBe('Only admins can change the branding.')
    expect(failureText($gettext, 'unsupported-type', 'logo')).toBe(
      'Use a PNG, JPEG, GIF or WebP image, or a simpler SVG.'
    )
  })

  it('adds nothing to the title when the cause is unknown', () => {
    expect(failureText($gettext, 'other', 'logo')).toBeUndefined()
  })
})
