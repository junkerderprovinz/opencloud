import { describe, expect, it } from 'vitest'
import { createGettext } from 'vue3-gettext'
import type { BrandingState } from '../../src/api'
import { backgroundNeedsRestart, failureText } from '../../src/feedback'

const { $gettext } = createGettext({ translations: {}, silent: true })

const state = (background: string, loginBackgroundActive: boolean): BrandingState => ({
  name: '',
  slogan: '',
  logo: '',
  logoDark: '',
  favicon: '',
  background,
  loginBackgroundActive
})

describe('backgroundNeedsRestart', () => {
  it('asks for a restart until the login page serves the saved background', () => {
    expect(backgroundNeedsRestart(state('/themes/_branding/background-1a2b3c.png', false))).toBe(true)
    expect(backgroundNeedsRestart(state('/themes/_branding/background-1a2b3c.png', true))).toBe(false)
  })

  it('asks for a restart until the login page drops a removed background', () => {
    expect(backgroundNeedsRestart(state('', true))).toBe(true)
    expect(backgroundNeedsRestart(state('', false))).toBe(false)
  })
})

describe('failureText', () => {
  it('names the limit of the image that was too large', () => {
    expect(failureText($gettext, 'too-large', 'favicon')).toBe('The file is larger than 2 MB.')
    expect(failureText($gettext, 'too-large', 'background')).toBe('The file is larger than 25 MB.')
  })

  it('explains missing admin rights and refused formats', () => {
    expect(failureText($gettext, 'forbidden')).toBe('Only admins can change the branding.')
    expect(failureText($gettext, 'unsupported-type', 'logo')).toBe('Use a PNG, JPEG, GIF, WebP or SVG image.')
  })

  it('adds nothing to the title when the cause is unknown', () => {
    expect(failureText($gettext, 'other', 'logo')).toBeUndefined()
  })
})
