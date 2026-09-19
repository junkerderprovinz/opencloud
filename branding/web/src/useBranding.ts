import { computed, ref } from 'vue'
import type { BrandingApi, BrandingState, ImageKind } from './api'

export function useBranding(api: BrandingApi, reloadTheme: () => Promise<void>) {
  const state = ref<BrandingState>()
  const name = ref('')
  const slogan = ref('')
  const busy = ref(false)

  const textChanged = computed(() => name.value !== state.value.name || slogan.value !== state.value.slogan)

  function showText(next: BrandingState) {
    name.value = next.name
    slogan.value = next.slogan
  }

  async function load() {
    state.value = await api.state()
    showText(state.value)
  }

  // Resolves to false when the change is saved but the open tab could not
  // pick up the new theme.
  async function change(save: () => Promise<BrandingState>) {
    busy.value = true
    try {
      state.value = await save()
      return await reloadTheme().then(
        () => true,
        (e) => {
          console.error(e)
          return false
        }
      )
    } finally {
      busy.value = false
    }
  }

  return {
    state,
    name,
    slogan,
    busy,
    textChanged,
    load,
    saveText: () =>
      change(async () => {
        const next = await api.saveText(name.value, slogan.value)
        showText(next)
        return next
      }),
    uploadImage: (kind: ImageKind, file: Blob) => change(() => api.uploadImage(kind, file)),
    clearImage: (kind: ImageKind) => change(() => api.clearImage(kind))
  }
}
