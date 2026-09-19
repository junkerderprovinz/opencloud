<template>
  <main class="ext:size-full ext:overflow-y-auto ext:p-4">
    <no-content-message v-if="loadFailure" :icon="loadFailure === 'forbidden' ? 'lock' : 'error-warning'">
      <template #message>
        <span
          v-text="
            loadFailure === 'forbidden'
              ? $gettext('Only admins can change the branding.')
              : $gettext('The branding could not be loaded.')
          "
        />
      </template>
    </no-content-message>
    <app-loading-spinner v-else-if="!state" />
    <div v-else class="ext:flex ext:max-w-2xl ext:flex-col ext:gap-8">
      <h1 class="ext:my-0 ext:text-2xl" v-text="$gettext('Branding')" />
      <section>
        <h2 class="ext:mt-0 ext:mb-2 ext:text-lg ext:font-semibold" v-text="$gettext('Name and slogan')" />
        <oc-text-input
          id="branding-name"
          v-model="name"
          class="ext:mb-3"
          :label="$gettext('Name')"
          :description-message="$gettext('Leave empty to use the OpenCloud default.')"
          :maxlength="64"
        />
        <oc-text-input
          id="branding-slogan"
          v-model="slogan"
          class="ext:mb-3"
          :label="$gettext('Slogan')"
          :description-message="$gettext('Leave empty to use the OpenCloud default.')"
          :maxlength="120"
        />
        <oc-button appearance="filled" :disabled="!textChanged || busy" @click="saveText">
          {{ $gettext('Save') }}
        </oc-button>
      </section>
      <image-section
        v-for="image in images"
        :key="image.kind"
        :kind="image.kind"
        :label="image.label"
        :preview="previewOf(image.kind, state, themeStore.availableThemes)"
        :busy="busy"
        :changing="changing === image.kind"
        @upload="upload(image, $event)"
        @reset="reset(image)"
      >
        <p
          v-if="image.kind === 'background' && backgroundNeedsRestart(state)"
          class="ext:text-sm"
          v-text="restartNote"
        />
      </image-section>
    </div>
  </main>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useGettext } from 'vue3-gettext'
import {
  AppLoadingSpinner,
  NoContentMessage,
  useClientService,
  useMessages,
  useThemeStore
} from '@opencloud-eu/web-pkg'
import { brandingApi, failureOf, type Failure, type ImageKind } from './api'
import { backgroundNeedsRestart, failureText } from './feedback'
import ImageSection from './ImageSection.vue'
import { previewOf } from './preview'
import { reloadTheme } from './reloadTheme'
import { useBranding } from './useBranding'

type ImageSlot = { kind: ImageKind; label: string }

const { $gettext } = useGettext()
const { showMessage, showErrorMessage } = useMessages()
const themeStore = useThemeStore()
const branding = useBranding(brandingApi(useClientService().httpAuthenticated), reloadTheme)
const { state, name, slogan, busy, textChanged } = branding
const loadFailure = ref<Failure>()
const changing = ref<ImageKind>()

const restartNote = $gettext('Restart the container once to apply the login background change.')

const images: ImageSlot[] = [
  { kind: 'logo', label: $gettext('Logo') },
  { kind: 'logo-dark', label: $gettext('Logo for dark mode') },
  { kind: 'favicon', label: $gettext('Favicon') },
  { kind: 'background', label: $gettext('Login background') }
]

async function report(change: Promise<boolean>, done: string, failed: string, kind?: ImageKind) {
  changing.value = kind
  try {
    const reloaded = await change
    const notes = [
      !reloaded && $gettext('Reload the page to see the change.'),
      kind === 'background' && backgroundNeedsRestart(state.value) && restartNote
    ].filter(Boolean)
    showMessage({ title: done, ...(notes.length > 0 && { desc: notes.join(' ') }) })
  } catch (e) {
    console.error(e)
    showErrorMessage({ title: failed, desc: failureText($gettext, failureOf(e), kind), errors: [e] })
  } finally {
    changing.value = undefined
  }
}

const saveText = () =>
  report(branding.saveText(), $gettext('Name and slogan saved'), $gettext('Name and slogan could not be saved'))

const upload = (image: ImageSlot, file: File) =>
  report(
    branding.uploadImage(image.kind, file),
    $gettext('%{image} saved', { image: image.label }),
    $gettext('%{image} could not be saved', { image: image.label }),
    image.kind
  )

const reset = (image: ImageSlot) =>
  report(
    branding.clearImage(image.kind),
    $gettext('%{image} reset to default', { image: image.label }),
    $gettext('%{image} could not be reset', { image: image.label }),
    image.kind
  )

onMounted(async () => {
  try {
    await branding.load()
  } catch (e) {
    console.error(e)
    loadFailure.value = failureOf(e)
  }
})
</script>
