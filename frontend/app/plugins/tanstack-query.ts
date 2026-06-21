import { watch } from 'vue'
import { VueQueryPlugin, QueryClient, hydrate, dehydrate } from '@tanstack/vue-query'
import { defineNuxtPlugin } from 'nuxt/app'
import {
  connectSSE,
  disconnectSSE,
  setSSEConfig,
  setSSEQueryClient,
} from '~/composables/useSSE'

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
    console.log('[PLUGIN] apiBase from runtimeConfig:', config.public.apiBase)
    const token = useCookie<string | null>('auth_token')
    setSSEQueryClient(queryClient)
    setSSEConfig(config.public.apiBase as string, () => token.value ?? null)
    watch(
      [() => config.public.apiBase, token],
      () => {
        if (token.value) connectSSE()
        else disconnectSSE()
      },
      { immediate: true },
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
