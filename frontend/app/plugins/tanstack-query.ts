import { watch } from 'vue'
import { VueQueryPlugin, QueryClient, hydrate, dehydrate } from '@tanstack/vue-query'
import { defineNuxtPlugin } from 'nuxt/app'
import { connectSSE, disconnectSSE, setSSEConfig, setSSEQueryClient } from '~/composables/useSSE'
import { useAuth } from '~/composables/useAuth'

export default defineNuxtPlugin((nuxtApp) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 5 * 60 * 1000, // 5 minutes
        refetchOnMount: false,
        refetchOnWindowFocus: false,
        retry: false,
      },
    },
  })

  nuxtApp.vueApp.use(VueQueryPlugin, { queryClient })

  if (import.meta.client) {
    const state = nuxtApp.payload.data.vueQueryState
    if (state) {
      hydrate(queryClient, state)
    }

    const config = useRuntimeConfig()
    const auth = useAuth()
    setSSEQueryClient(queryClient)
    setSSEConfig(config.public.apiBase as string, () => auth.token.value ?? null)
    watch(
      [() => config.public.apiBase, () => auth.token.value],
      () => {
        if (auth.token.value) connectSSE()
        else disconnectSSE()
      },
      { immediate: true }
    )
  }

  if (import.meta.server) {
    nuxtApp.hooks.hook('app:rendered', () => {
      nuxtApp.payload.data.vueQueryState = dehydrate(queryClient)
    })
  }

  return {
    provide: {
      queryClient,
    },
  }
})
