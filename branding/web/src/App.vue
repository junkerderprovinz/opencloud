<template>
  <main class="branding-app">
    <h1 v-text="$gettext('Branding')" />
    <p v-if="error" class="branding-error" role="alert" v-text="error" />

    <section v-if="state" class="branding-section">
      <h2 v-text="$gettext('Name and slogan')" />
      <oc-text-input id="branding-name" v-model="name" :label="$gettext('Name')" :maxlength="64" />
      <oc-text-input id="branding-slogan" v-model="slogan" :label="$gettext('Slogan')" :maxlength="120" />
      <oc-button appearance="filled" :disabled="busy" @click="saveText">
        {{ $gettext('Save') }}
      </oc-button>
    </section>

    <section v-if="state" class="branding-section">
      <h2 v-text="$gettext('Images')" />
      <div v-for="slot in slots" :key="slot.kind" class="branding-slot">
        <h3 v-text="slot.label" />
        <img v-if="state[slot.field]" :src="state[slot.field]" :alt="slot.label" class="branding-preview" />
        <p v-else v-text="$gettext('OpenCloud default')" />
        <input type="file" :accept="acceptedTypes.join(',')" :disabled="busy" @change="upload(slot.kind, $event)" />
        <oc-button v-if="state[slot.field]" appearance="outline" :disabled="busy" @click="clear(slot.kind)">
          {{ $gettext('Reset') }}
        </oc-button>
      </div>
      <p
        v-if="backgroundNeedsRestart"
        class="branding-note"
        v-text="$gettext('Restart the container once to apply the login background change.')"
      />
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useGettext } from 'vue3-gettext'
import { useClientService } from '@opencloud-eu/web-pkg'
import { acceptedTypes, brandingApi, validateUpload, type BrandingState, type ImageKind } from './api'
import { reloadTheme } from './reloadTheme'

type ImageField = 'logo' | 'logoDark' | 'favicon' | 'background'

const { $gettext } = useGettext()
const api = brandingApi(useClientService().httpAuthenticated)

const state = ref<BrandingState>()
const name = ref('')
const slogan = ref('')
const busy = ref(false)
const error = ref('')

const slots: { kind: ImageKind; field: ImageField; label: string }[] = [
  { kind: 'logo', field: 'logo', label: $gettext('Logo') },
  { kind: 'logo-dark', field: 'logoDark', label: $gettext('Logo for dark mode') },
  { kind: 'favicon', field: 'favicon', label: $gettext('Favicon') },
  { kind: 'background', field: 'background', label: $gettext('Login background') }
]

// The IDP reads its background URL only at startup, so the first background
// and its removal both need one restart.
const backgroundNeedsRestart = computed(
  () => !!state.value && !!state.value.background !== state.value.loginBackgroundActive
)

function apply(next: BrandingState) {
  state.value = next
  name.value = next.name
  slogan.value = next.slogan
}

async function run(action: () => Promise<BrandingState>) {
  busy.value = true
  error.value = ''
  try {
    apply(await action())
    await reloadTheme()
  } catch (e) {
    console.error(e)
    error.value = $gettext('Saving failed. Check the file and try again.')
  } finally {
    busy.value = false
  }
}

const saveText = () => run(() => api.saveText(name.value, slogan.value))
const clear = (kind: ImageKind) => run(() => api.clearImage(kind))

function upload(kind: ImageKind, event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) {
    return
  }
  const problem = validateUpload(kind, file)
  if (problem === 'too-large') {
    error.value = $gettext('The file is too large.')
    return
  }
  if (problem === 'unsupported-type') {
    error.value = $gettext('Use PNG, JPEG, GIF, WebP or SVG.')
    return
  }
  return run(() => api.uploadImage(kind, file))
}

onMounted(async () => {
  try {
    apply(await api.state())
  } catch (e) {
    console.error(e)
    error.value = $gettext('Only admins can change the branding.')
  }
})
</script>

<style scoped>
.branding-app {
  max-width: 48rem;
  padding: 1.5rem;
  overflow-y: auto;
}
.branding-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 1.5rem;
}
.branding-slot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.75rem;
}
.branding-slot h3 {
  width: 100%;
  margin: 0;
}
.branding-preview {
  max-height: 4rem;
  max-width: 12rem;
  object-fit: contain;
}
.branding-error {
  color: var(--oc-role-error);
}
</style>
