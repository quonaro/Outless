/// <reference types="nuxt" />
import { useAuth } from '~/composables/useAuth'

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  const { apiBase } = config.public

  // Capture auth composable inside plugin context so token is readable
  // even when onRequest/onResponseError run without Nuxt context during SSR.
  const auth = useAuth()

  // Dedicated API client with auth interceptor.
  const $api = $fetch.create({
    baseURL: apiBase,
    onRequest({ options }) {
      // Add auth token if available
      const token = auth.token.value
      if (token) {
        const headers = new Headers(options.headers as HeadersInit)
        headers.set('Authorization', `Bearer ${token}`)
        options.headers = headers
      }
    },
    onResponseError({ response }) {
      // Handle 401 unauthorized - clear token and redirect to login
      if (response.status === 401) {
        auth.clearToken()
        if (import.meta.client) {
          navigateTo('/login')
        }
      }
    },
  })

  // Provide $api to components/composables that need it explicitly.
  return {
    provide: {
      api: $api,
    },
  }
})
