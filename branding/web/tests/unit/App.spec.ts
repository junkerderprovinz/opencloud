import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createGettext } from 'vue3-gettext'
import type { WebThemeType } from '@opencloud-eu/web-pkg'
import type { BrandingState, ImageKind } from '../../src/api'
import App from '../../src/App.vue'
import ImageSection from '../../src/ImageSection.vue'
import { reloadTheme } from '../../src/reloadTheme'

const { http, messages } = vi.hoisted(() => ({
  http: { get: vi.fn(), put: vi.fn(), request: vi.fn() },
  messages: { showMessage: vi.fn(), showErrorMessage: vi.fn() }
}))

vi.mock('@opencloud-eu/web-pkg', async () => {
  const { defineComponent, h } = await import('vue')
  return {
    useMessages: () => messages,
    useClientService: () => ({ httpAuthenticated: http }),
    useThemeStore: () => ({ availableThemes: [] as WebThemeType[] }),
    AppLoadingSpinner: defineComponent(() => () => h('span')),
    NoContentMessage: defineComponent((_, { slots }) => () => h('div', slots.message?.()))
  }
})
vi.mock('../../src/reloadTheme', () => ({ reloadTheme: vi.fn() }))

const saved: BrandingState = {
  name: 'Knight Cloud',
  slogan: 'Files, forged',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginBackgroundActive: false
}
const restartNote = 'Restart the container once to apply the login background change.'

const answered = (status: number) => Object.assign(new Error(`status ${status}`), { response: { status } })

async function mountApp() {
  const wrapper = mount(App, {
    global: {
      plugins: [createGettext({ translations: {}, silent: true })],
      stubs: { OcTextInput: true, OcButton: true, ImageSection: true },
      renderStubDefaultSlot: true
    }
  })
  await flushPromises()
  return wrapper
}

async function upload(wrapper: VueWrapper, kind: ImageKind, answer: BrandingState) {
  http.put.mockResolvedValueOnce({ data: answer })
  const section = wrapper.findAllComponents(ImageSection).find((section) => section.props('kind') === kind)
  section.vm.$emit('upload', new File(['x'], `${kind}.png`))
  await flushPromises()
}

enableAutoUnmount(afterEach)

describe('App', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    http.get.mockResolvedValue({ data: saved })
    vi.mocked(reloadTheme).mockResolvedValue()
  })

  afterEach(() => {
    vi.resetAllMocks()
    vi.restoreAllMocks()
  })

  it.each([
    ['refused', () => Promise.reject(answered(403)), 'Only admins can change the branding.'],
    ['failed', () => Promise.reject(answered(500)), 'The branding could not be loaded.'],
    ['answered with a page', async () => ({ data: '<!DOCTYPE html><html></html>' }), 'The branding could not be loaded.']
  ])('explains a load that was %s', async (_, answer, text) => {
    http.get.mockImplementation(answer)

    const wrapper = await mountApp()

    expect(wrapper.text()).toBe(text)
  })

  it('confirms a change the open tab already shows', async () => {
    const wrapper = await mountApp()

    await upload(wrapper, 'logo', { ...saved, logo: '/themes/_branding/logo-1a2b3c4d5e6f.png' })

    expect(messages.showMessage).toHaveBeenCalledWith({ title: 'Logo saved' })
  })

  it('asks for a page reload when the open tab missed the new theme', async () => {
    vi.mocked(reloadTheme).mockRejectedValue(new Error('theme reload failed: 502'))
    const wrapper = await mountApp()

    await upload(wrapper, 'logo', { ...saved, logo: '/themes/_branding/logo-1a2b3c4d5e6f.png' })

    expect(messages.showMessage).toHaveBeenCalledWith({ title: 'Logo saved', desc: 'Reload the page to see the change.' })
  })

  it('asks for a restart until the login page shows the saved background', async () => {
    const wrapper = await mountApp()
    expect(wrapper.text()).not.toContain(restartNote)

    await upload(wrapper, 'background', { ...saved, background: '/themes/_branding/background-1a2b3c4d5e6f.png' })

    expect(messages.showMessage).toHaveBeenCalledWith({ title: 'Login background saved', desc: restartNote })
    expect(wrapper.text()).toContain(restartNote)
  })
})
