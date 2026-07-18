/// <reference types="nuxt" />

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  const { apiBase } = config.public

  const auth = useAuth()

  // Dedicated API client with auth interceptor.
  const $api = $fetch.create({
    baseURL: apiBase,
    credentials: 'include',
    onResponseError({ response }) {
      if (response.status !== 401) return
      // Skip if this is already a logout/auth request to avoid loops
      const url = response.url ?? ''
      if (url.includes('/auth/logout') || url.includes('/auth/me')) return
      // Clear user locally without calling logout API (avoid loop)
      auth.clearUser()
      if (import.meta.client) {
        navigateTo('/login')
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
