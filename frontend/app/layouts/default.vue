<script setup lang="ts">
import { computed, watch } from 'vue'
import { Menu } from 'lucide-vue-next'
import Sidebar from '~/components/Sidebar.vue'
import { Toaster } from 'vue-sonner'
import { useAuth } from '~/composables/useAuth'
import { useSidebar } from '~/composables/useSidebar'

const colorMode = useNuxtApp().$colorMode
const auth = useAuth()
const sidebar = useSidebar()

const toasterTheme = computed<'light' | 'dark'>(() => {
  return colorMode.value === 'dark' ? 'dark' : 'light'
})

watch(
  auth.isAuthenticated,
  (isAuthenticated) => {
    if (!isAuthenticated && import.meta.client) {
      navigateTo('/login')
    }
  },
  { immediate: true }
)
</script>

<template>
  <div>
    <div class="flex h-screen bg-background">
      <Sidebar />
      <div class="flex flex-1 flex-col overflow-hidden">
        <!-- Mobile header -->
        <header
          class="flex items-center justify-between border-b border-border bg-background px-4 py-3 md:hidden"
        >
          <button
            class="rounded-md p-2 text-muted-foreground hover:bg-accent hover:text-foreground"
            @click="sidebar.toggleMobile()"
          >
            <Menu class="h-5 w-5" />
          </button>
          <span class="font-semibold text-foreground">Outless</span>
          <div class="w-9" />
        </header>
        <main class="flex-1 overflow-y-auto">
          <slot />
        </main>
      </div>
    </div>
    <Toaster
      position="top-right"
      :theme="toasterTheme"
      :toast-options="{
        style: {
          background: 'hsl(var(--background))',
          border: '1px solid hsl(var(--border))',
          color: 'hsl(var(--foreground))',
        },
      }"
    />
  </div>
</template>
