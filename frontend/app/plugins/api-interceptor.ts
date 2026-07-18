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
      // Handle 401 unauthorized - clear user and redirect to login
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
