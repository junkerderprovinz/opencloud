import { describe, expect, it, vi } from 'vitest'
import { createApp } from 'vue'
import { createGettext } from 'vue3-gettext'
import branding from '../../src/index'

const granted = vi.hoisted(() => new Set<string>())

vi.mock('@opencloud-eu/web-pkg', () => ({
  defineWebApplication: <T>(app: T) => app,
  useAbility: () => ({ can: (action: string, subject: string) => granted.has(`${action} ${subject}`) })
}))
vi.mock('../../src/App.vue', () => ({ default: {} }))

function menuItems(...abilities: string[]) {
  granted.clear()
  abilities.forEach((ability) => granted.add(ability))
  const app = createApp({}).use(createGettext({ translations: {}, silent: true }))
  const { extensions } = app.runWithContext(() => branding.setup({ applicationConfig: {} }))
  return extensions.value.map((extension) => extension.id)
}

describe('app menu entry', () => {
  it('is listed for a user who may change the logo', () => {
    expect(menuItems('update-all Logo')).toEqual(['app.branding.menuItem'])
  })

  it('stays hidden for everyone else', () => {
    expect(menuItems()).toEqual([])
    expect(menuItems('read-all Logo', 'update-all Account')).toEqual([])
  })
})
