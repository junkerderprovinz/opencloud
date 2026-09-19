<template>
  <section>
    <div class="ext:flex ext:items-center">
      <h2 class="ext:my-0 ext:text-lg ext:font-semibold" v-text="label" />
      <oc-button
        :id="menuId"
        v-oc-tooltip="$gettext('Show context menu')"
        :aria-label="$gettext('Show context menu for %{image}', { image: label })"
        appearance="raw"
        class="ext:ml-2 ext:p-1"
      >
        <oc-icon name="more-2" />
      </oc-button>
      <oc-drop :drop-id="`${menuId}-drop`" :toggle="`#${menuId}`" mode="click" close-on-click padding-size="small">
        <context-action-menu :menu-sections="menuSections" :action-options="{}" />
      </oc-drop>
    </div>
    <div
      class="ext:mt-2 ext:flex ext:aspect-video ext:w-[280px] ext:max-w-full ext:items-center ext:justify-center ext:overflow-hidden ext:rounded-xl ext:border"
      :class="previewClass"
    >
      <img v-if="url" :src="url" :alt="label" :class="imageClass" />
      <span v-else v-text="$gettext('OpenCloud default')" />
    </div>
    <p class="ext:text-sm ext:text-role-on-surface-variant" v-text="hint" />
    <slot />
    <input
      ref="input"
      type="file"
      class="ext:hidden"
      :aria-label="$gettext('Upload %{image}', { image: label })"
      :accept="acceptedTypes.join(',')"
      @change="picked"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useGettext } from 'vue3-gettext'
import { ContextActionMenu, type Action } from '@opencloud-eu/web-pkg'
import { acceptedTypes, limits, MB, type ImageKind } from './api'

const { kind, label, url, busy } = defineProps<{ kind: ImageKind; label: string; url: string; busy: boolean }>()
const emit = defineEmits<{ upload: [file: File]; reset: [] }>()

const { $gettext } = useGettext()
const input = ref<HTMLInputElement>()
const menuId = `branding-${kind}-menu`

const hint = $gettext('PNG, JPEG, GIF, WebP or SVG, up to %{size} MB', { size: String(limits[kind] / MB) })

// Each logo is previewed on the kind of background it is made for, not on the current theme,
// so its placeholder text needs a fixed colour as well.
const previewClass = {
  logo: 'ext:bg-white ext:text-neutral-600 ext:p-2',
  'logo-dark': 'ext:bg-neutral-900 ext:text-neutral-300 ext:p-2',
  favicon: 'ext:bg-role-surface-container ext:text-role-on-surface-variant ext:p-2',
  background: 'ext:bg-role-surface-container ext:text-role-on-surface-variant'
}[kind]
const imageClass =
  kind === 'background' ? 'ext:size-full ext:object-cover' : 'ext:max-h-full ext:max-w-full ext:object-contain'

// The drop removes the clicked menu item, so focus goes back to the menu button.
const focusMenuButton = () => document.getElementById(menuId).focus()

const actions: Action[] = [
  {
    name: 'upload',
    icon: 'image-add',
    label: () => $gettext('Upload image'),
    isVisible: () => true,
    isDisabled: () => busy,
    handler: () => {
      focusMenuButton()
      input.value.click()
    }
  },
  {
    name: 'reset',
    icon: 'restart',
    label: () => $gettext('Reset to default'),
    isVisible: () => !!url,
    isDisabled: () => busy,
    handler: () => {
      focusMenuButton()
      emit('reset')
    }
  }
]

const menuSections = computed(() => [{ name: 'primaryActions', items: actions.filter((action) => action.isVisible()) }])

function picked() {
  const file = input.value.files[0]
  input.value.value = ''
  if (file) {
    emit('upload', file)
  }
}
</script>
