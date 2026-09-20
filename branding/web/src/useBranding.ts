import { computed, ref } from 'vue'
import type { BrandingApi, BrandingState, ImageKind, LoginTheme } from './api'

export function useBranding(api: BrandingApi, reloadTheme: () => Promise<void>) {
  const state = ref<BrandingState>()
  const name = ref('')
  const slogan = ref('')
  const loginTheme = ref<LoginTheme>('')
  const busy = ref(false)

  const textChanged = computed(() => name.value !== state.value.name || slogan.value !== state.value.slogan)

  function showText(next: BrandingState) {
    name.value = next.name
    slogan.value = next.slogan
  }

  async function load() {
    state.value = await api.state()
    showText(state.value)
    loginTheme.value = state.value.loginTheme
  }

  async function save(next: () => Promise<BrandingState>) {
    const before = state.value
    state.value = await next()
    // Another tab may have saved a new name or slogan since this one loaded.
    if (name.value === before.name) {
      name.value = state.value.name
    }
    if (slogan.value === before.slogan) {
      slogan.value = state.value.slogan
    }
    loginTheme.value = state.value.loginTheme
  }

  // Resolves to false when the change is saved but the open tab could not
  // pick up the new theme.
  async function change(next: () => Promise<BrandingState>) {
    busy.value = true
    try {
      await save(next)
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

  // The sign-in card is not part of this tab's theme, so there is nothing to
  // reload here. The choice shows at once and falls back if the save fails.
  async function saveLoginTheme(next: LoginTheme) {
    const before = loginTheme.value
    loginTheme.value = next
    busy.value = true
    try {
      await save(() => api.saveLoginTheme(next))
      return true
    } catch (e) {
      loginTheme.value = before
      throw e
    } finally {
      busy.value = false
    }
  }

  return {
    state,
    name,
    slogan,
    loginTheme,
    busy,
    textChanged,
    load,
    saveText: () =>
      change(async () => {
        const next = await api.saveText(name.value, slogan.value)
        showText(next)
        return next
      }),
    saveLoginTheme,
    uploadImage: (kind: ImageKind, file: Blob) => change(() => api.uploadImage(kind, file)),
    clearImage: (kind: ImageKind) => change(() => api.clearImage(kind))
  }
}
