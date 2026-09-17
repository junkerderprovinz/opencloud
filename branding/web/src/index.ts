import { AppMenuItemExtension, defineWebApplication, useAbility } from '@opencloud-eu/web-pkg'
import { urlJoin } from '@opencloud-eu/web-client'
import { computed } from 'vue'
import { useGettext } from 'vue3-gettext'
import App from './App.vue'

const appId = 'branding'

export default defineWebApplication({
  setup() {
    const { $gettext } = useGettext()
    const { can } = useAbility()

    const appInfo = {
      name: $gettext('Branding'),
      id: appId,
      icon: 'palette'
    }

    const routes = [
      {
        path: '/',
        name: appId,
        component: App,
        meta: {
          authContext: 'user',
          title: $gettext('Branding')
        }
      }
    ]

    // Hiding the entry is convenience only; brandingd checks the permission.
    const extensions = computed<AppMenuItemExtension[]>(() =>
      can('update-all', 'Logo')
        ? [
            {
              id: `app.${appId}.menuItem`,
              type: 'appMenuItem',
              label: () => appInfo.name,
              icon: appInfo.icon,
              priority: 90,
              path: urlJoin(appId)
            }
          ]
        : []
    )

    return {
      appInfo,
      routes,
      extensions
    }
  }
})
