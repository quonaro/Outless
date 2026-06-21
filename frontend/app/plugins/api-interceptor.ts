/// <reference types="nuxt" />
import { useAuth } from '~/composables/useAuth'

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  const { apiBase } = config.public

  console.log('[API-INTERCEPTOR] Plugin initialized, apiBase:', apiBase)

  // Dedicated API client with auth interceptor.
  const $api = $fetch.create({
    baseURL: apiBase,
    onRequest({ options }) {
      // Add auth token if available
      const auth = useAuth()
      console.log('[API-INTERCEPTOR] onRequest - token exists:', !!auth.token.value, 'token:', auth.token.value?.substring(0, 20) + '...')
      if (auth.token.value) {
        const headers = new Headers(options.headers as HeadersInit)
        headers.set('Authorization', `Bearer ${auth.token.value}`)
        options.headers = headers
        console.log('[API-INTERCEPTOR] Authorization header set')
      } else {
        console.log('[API-INTERCEPTOR] No token available')
      }
    },
    onResponseError({ response }) {
      // Handle 401 unauthorized - clear token and redirect to login
      if (response.status === 401) {
        const auth = useAuth()
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
