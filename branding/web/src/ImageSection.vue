<template>
  <section :aria-busy="changing || undefined" @keydown.capture="noteInput" @pointerdown.capture="noteInput">
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
      <oc-drop
        :drop-id="`${menuId}-drop`"
        :toggle="`#${menuId}`"
        :title="label"
        mode="click"
        close-on-click
        padding-size="small"
      >
        <context-action-menu :menu-sections="menuSections" :action-options="{}" />
      </oc-drop>
    </div>
    <!-- aria-disabled rather than disabled, so a keyboard user keeps the focus while the upload runs -->
    <button
      type="button"
      class="ext:mt-2 ext:flex ext:aspect-video ext:w-[280px] ext:max-w-full ext:cursor-pointer ext:items-center ext:justify-center ext:overflow-hidden ext:rounded-xl ext:border ext:aria-disabled:cursor-default"
      :class="previewClass"
      :style="preview.chrome && { backgroundColor: preview.chrome.background, color: preview.chrome.color }"
      :aria-label="uploadLabel"
      :aria-disabled="busy || undefined"
      @click="choose"
    >
      <oc-spinner v-if="changing" size="large" :aria-label="$gettext('Saving...')" />
      <img v-else-if="preview.url" :src="preview.url" :alt="label" :class="imageClass" />
      <span v-else v-text="$gettext('OpenCloud default')" />
    </button>
    <p v-if="caption" class="ext:text-sm ext:text-role-on-surface-variant" v-text="caption" />
    <p class="ext:text-sm ext:text-role-on-surface-variant" v-text="hint" />
    <input
      ref="input"
      type="file"
      class="ext:hidden"
      :aria-label="uploadLabel"
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
import type { Preview } from './preview'

const { kind, label, preview, busy, changing } = defineProps<{
  kind: ImageKind
  label: string
  preview: Preview
  busy: boolean
  changing: boolean
}>()
const emit = defineEmits<{ upload: [file: File]; reset: [] }>()

const { $gettext } = useGettext()
const input = ref<HTMLInputElement>()
const menuId = `branding-${kind}-menu`

const hint = $gettext('PNG, JPEG, GIF, WebP or SVG, up to %{size} MB', { size: String(limits[kind] / MB) })
const uploadLabel = $gettext('Upload image: %{image}', { image: label })

// A logo is previewed on the top bar of the theme it is made for, not on the current theme.
// These colours stand in when that theme names none.
const previewClass = {
  logo: 'ext:bg-white ext:text-neutral-600 ext:p-2',
  'logo-dark': 'ext:bg-neutral-900 ext:text-neutral-300 ext:p-2',
  favicon: 'ext:bg-role-surface-container ext:text-role-on-surface-variant ext:p-2',
  background: 'ext:bg-role-surface-container ext:text-role-on-surface-variant'
}[kind]
const imageClass =
  kind === 'background' ? 'ext:size-full ext:object-cover' : 'ext:max-h-full ext:max-w-full ext:object-contain'

const caption = computed(() => {
  if (preview.source === 'logo') {
    return $gettext('Uses the logo')
  }
  return preview.source === 'default' && preview.url ? $gettext('OpenCloud default') : ''
})

// The drop removes the chosen item, so the focus goes back to the menu button. After a
// pointer press it would open the button's tooltip instead.
let usedPointer = false
const noteInput = (event: Event) => {
  usedPointer = event.type === 'pointerdown'
}
function focusMenuButton() {
  if (!usedPointer) {
    document.getElementById(menuId).focus()
  }
}

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
    isVisible: () => preview.source === 'custom',
    isDisabled: () => busy,
    handler: () => {
      focusMenuButton()
      emit('reset')
    }
  }
]

const menuSections = computed(() => [{ name: 'primaryActions', items: actions.filter((action) => action.isVisible()) }])

function choose() {
  if (!busy) {
    input.value.click()
  }
}

function picked() {
  const file = input.value.files[0]
  input.value.value = ''
  if (file) {
    emit('upload', file)
  }
}
</script>
