import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createGettext } from 'vue3-gettext'
import type { Action } from '@opencloud-eu/web-pkg'
import type { BrandingState } from '../../src/api'
import ImageSection from '../../src/ImageSection.vue'
import { previewOf, type Preview } from '../../src/preview'

function passThrough(tag: string) {
  return defineComponent((_, { slots }) => {
    return () => h(tag, slots.default?.())
  })
}

function mountSection(props: {
  kind?: 'logo' | 'logo-dark'
  label?: string
  preview: Preview
  busy?: boolean
  changing?: boolean
}) {
  return mount(ImageSection, {
    props: { kind: 'logo', label: 'Logo', busy: false, changing: false, ...props },
    global: {
      plugins: [createGettext({ translations: {}, silent: true })],
      components: {
        OcButton: passThrough('button'),
        OcDrop: passThrough('div'),
        OcIcon: passThrough('i'),
        OcSpinner: passThrough('span')
      },
      directives: { OcTooltip: {} },
      stubs: { ContextActionMenu: true }
    }
  })
}

const actions = (wrapper: VueWrapper) =>
  wrapper.findComponent({ name: 'ContextActionMenu' }).props('menuSections')[0].items as Action[]
const actionNames = (wrapper: VueWrapper) => actions(wrapper).map((action) => action.name)

const logo = '/themes/_branding/logo-1a2b3c4d5e6f.png'
const logoDark = '/themes/_branding/logo-dark-6f5e4d3c2b1a.svg'
const nothingSaved: BrandingState = {
  name: '',
  slogan: '',
  logo: '',
  logoDark: '',
  favicon: '',
  background: '',
  loginBackgroundActive: false
}

describe('ImageSection', () => {
  it('offers Reset only for an image saved in this slot', () => {
    const stock = mountSection({ preview: { url: 'themes/opencloud/assets/logo.svg', source: 'default' } })
    const own = mountSection({ preview: { url: logo, source: 'custom' } })

    expect(actionNames(stock)).toEqual(['upload'])
    expect(actionNames(own)).toEqual(['upload', 'reset'])
  })

  it('disables Upload and Reset while a change runs', () => {
    const idle = actions(mountSection({ preview: { url: logo, source: 'custom' } }))
    const busy = actions(mountSection({ preview: { url: logo, source: 'custom' }, busy: true }))

    expect(idle.map((action) => action.isDisabled())).toEqual([false, false])
    expect(busy.map((action) => action.isDisabled())).toEqual([true, true])
  })

  it('shows the light logo in the dark slot and offers no Reset for it', () => {
    const fallback = mountSection({
      kind: 'logo-dark',
      label: 'Logo for dark mode',
      preview: previewOf('logo-dark', { ...nothingSaved, logo }, [])
    })
    const own = mountSection({
      kind: 'logo-dark',
      label: 'Logo for dark mode',
      preview: previewOf('logo-dark', { ...nothingSaved, logo, logoDark }, [])
    })

    expect(fallback.find('img').attributes('src')).toBe(logo)
    expect(fallback.text()).toContain('Uses the logo')
    expect(actionNames(fallback)).toEqual(['upload'])
    expect(own.find('img').attributes('src')).toBe(logoDark)
    expect(own.text()).not.toContain('Uses the logo')
    expect(actionNames(own)).toEqual(['upload', 'reset'])
  })

  it('names a stock image as the OpenCloud default', () => {
    const stock = mountSection({ preview: { url: 'themes/opencloud/assets/logo.svg', source: 'default' } })
    const none = mountSection({ preview: { url: '', source: 'default' } })

    expect(stock.find('img').attributes('src')).toBe('themes/opencloud/assets/logo.svg')
    expect(stock.text()).toContain('OpenCloud default')
    expect(none.find('img').exists()).toBe(false)
    expect(none.text()).toContain('OpenCloud default')
  })

  it('previews a logo on the top bar colours it gets', () => {
    const wrapper = mountSection({
      preview: { url: logo, source: 'custom', chrome: { background: '#20434f', color: '#ffffff' } }
    })

    const box = wrapper.find('img').element.parentElement
    expect(box.style.backgroundColor).toBe('#20434f')
    expect(box.style.color).toBe('#ffffff')
  })

  it('shows progress only in the slot being changed', () => {
    const changing = mountSection({ preview: { url: logo, source: 'custom' }, busy: true, changing: true })
    const waiting = mountSection({ preview: { url: logo, source: 'custom' }, busy: true })

    expect(changing.find('section').attributes('aria-busy')).toBe('true')
    expect(changing.find('[aria-label="Saving..."]').exists()).toBe(true)
    expect(changing.find('img').exists()).toBe(false)
    expect(waiting.find('section').attributes('aria-busy')).toBeUndefined()
    expect(waiting.find('[aria-label="Saving..."]').exists()).toBe(false)
    expect(waiting.find('img').attributes('src')).toBe(logo)
  })
})
