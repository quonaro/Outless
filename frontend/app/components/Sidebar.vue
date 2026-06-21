<script setup lang="ts">
import { LayoutDashboard, Key, Globe, Settings, LogOut, Server } from 'lucide-vue-next'

import { useSidebar } from '~/composables/useSidebar'
import { useAuth } from '~/composables/useAuth'
import { useSSEStatus } from '~/composables/useSSE'
import logoImage from '~/assets/img/logo-d-a.webp'
import ThemeToggle from './ThemeToggle.vue'

const sidebar = useSidebar()
const auth = useAuth()
const route = useRoute()
const { isBackendAvailable, isConnecting } = useSSEStatus()

const navItems = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: LayoutDashboard,
    path: '/dashboard',
  },
  { id: 'tokens', label: 'Tokens', icon: Key, path: '/tokens' },
  { id: 'nodes', label: 'Nodes', icon: Globe, path: '/nodes' },
  { id: 'inbounds', label: 'Inbounds', icon: Server, path: '/inbounds' },
  { id: 'settings', label: 'Settings', icon: Settings, path: '/settings' },
]

const activeItem = computed(() => {
  const activeNav = navItems.find((item) => route.path === item.path)
  return activeNav?.id || 'dashboard'
})

const handleNavClick = (path: string) => {
  navigateTo(path)
}

const handleLogout = () => {
  auth.clearToken()
  navigateTo('/login')
}
</script>

<template>
  <aside
    class="h-screen flex flex-col border-r bg-background transition-all duration-300"
    :class="sidebar.isExpanded ? 'w-72' : 'w-20'"
  >
    <!-- Logo Section -->
    <div class="p-4 border-b border-border">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <img :src="logoImage" alt="Outless Logo" class="w-12 h-12 flex-shrink-0" />
          <span v-if="sidebar.isExpanded" class="font-bold text-lg text-foreground">
            Outless
            <!-- SSE state differs SSR vs client; avoid hydration mismatch -->
            <ClientOnly>
              <span
                class="ml-2 inline-block h-2.5 w-2.5 rounded-full"
                :class="isBackendAvailable ? 'bg-emerald-500' : 'bg-red-500'"
                :title="
                  isBackendAvailable
                    ? 'Backend available'
                    : isConnecting
                      ? 'Connecting to backend...'
                      : 'Backend unavailable'
                "
              />
              <template #fallback>
                <span
                  class="ml-2 inline-block h-2.5 w-2.5 rounded-full bg-muted-foreground/40"
                  title="Checking backend connection..."
                />
              </template>
            </ClientOnly>
          </span>
        </div>
        <ThemeToggle />
      </div>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 p-4 space-y-2 overflow-y-auto">
      <template v-for="item in navItems" :key="item.id">
        <div class="relative group">
          <!-- Main Nav Item -->
          <button
            class="w-full flex items-center justify-between p-3 rounded-lg transition-colors"
            :class="
              activeItem === item.id
                ? 'bg-primary/10 text-primary dark:bg-primary/20'
                : 'hover:bg-accent text-muted-foreground hover:text-foreground'
            "
            @click="handleNavClick(item.path)"
          >
            <div class="flex items-center gap-3">
              <component :is="item.icon" class="w-5 h-5 flex-shrink-0" />
              <span v-if="sidebar.isExpanded" class="font-medium">{{ item.label }}</span>
            </div>
          </button>
        </div>
      </template>
    </nav>

    <!-- Logout Button -->
    <div class="p-4 border-t border-border">
      <button
        class="w-full flex items-center gap-3 p-3 rounded-lg transition-colors hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
        @click="handleLogout"
      >
        <LogOut class="w-5 h-5 flex-shrink-0" />
        <span v-if="sidebar.isExpanded" class="font-medium">Logout</span>
      </button>
    </div>
  </aside>
</template>
