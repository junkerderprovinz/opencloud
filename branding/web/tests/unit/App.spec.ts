import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
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
  loginTheme: ''
}

const answered = (status: number) => Object.assign(new Error(`status ${status}`), { response: { status } })

// Stands in for the design system's radio, which the test environment cannot
// resolve. It keeps the boolean model type of the original, which makes Vue
// read an empty option as true.
const OcRadio = defineComponent({
  props: { label: { type: String }, option: { type: String }, modelValue: { type: Boolean } },
  emits: ['update:modelValue'],
  setup: (props, { emit }) => () =>
    h(
      'button',
      {
        'aria-pressed': String((props.modelValue as unknown) === props.option),
        onClick: () => emit('update:modelValue', props.option)
      },
      props.label
    )
})

async function mountApp() {
  const wrapper = mount(App, {
    global: {
      plugins: [createGettext({ translations: {}, silent: true })],
      stubs: { OcTextInput: true, OcButton: true, ImageSection: true, OcRadio }
    }
  })
  await flushPromises()
  return wrapper
}

const card = (wrapper: VueWrapper, label: string) => wrapper.findAll('button').find((one) => one.text() === label)

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

  it('offers the three looks of the sign-in card and marks the saved one', async () => {
    http.get.mockResolvedValue({ data: { ...saved, loginTheme: 'auto' } })

    const wrapper = await mountApp()

    expect(wrapper.findAll('button').map((one) => one.text())).toEqual([
      'Light, the OpenCloud default',
      'Dark',
      'Follow the browser'
    ])
    expect(card(wrapper, 'Follow the browser').attributes('aria-pressed')).toBe('true')
  })

  it('saves a chosen sign-in card at once', async () => {
    const wrapper = await mountApp()
    http.put.mockResolvedValueOnce({ data: { ...saved, loginTheme: 'dark' } })

    await card(wrapper, 'Dark').trigger('click')
    await flushPromises()

    expect(http.put).toHaveBeenCalledWith(
      'brandingsvc/api/login-theme',
      { loginTheme: 'dark' },
      { headers: { 'X-Branding-Request': '1' } }
    )
    expect(messages.showMessage).toHaveBeenCalledWith({ title: 'Login page saved' })
    expect(card(wrapper, 'Dark').attributes('aria-pressed')).toBe('true')
  })

  it('keeps the saved sign-in card when the choice cannot be saved', async () => {
    const wrapper = await mountApp()
    http.put.mockRejectedValueOnce(answered(403))

    await card(wrapper, 'Dark').trigger('click')
    await flushPromises()

    expect(messages.showErrorMessage).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Login page could not be saved' })
    )
    expect(card(wrapper, 'Light, the OpenCloud default').attributes('aria-pressed')).toBe('true')
  })

  it('confirms a saved login background', async () => {
    const wrapper = await mountApp()

    await upload(wrapper, 'background', { ...saved, background: '/themes/_branding/background-1a2b3c4d5e6f.png' })

    expect(messages.showMessage).toHaveBeenCalledWith({ title: 'Login background saved' })
  })
})
