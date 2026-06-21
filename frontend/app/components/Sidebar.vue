<script setup lang="ts">
import { LayoutDashboard, Key, Globe, Settings, LogOut, Server, X } from 'lucide-vue-next'

import { useSidebar } from '~/composables/useSidebar'
import { useAuth } from '~/composables/useAuth'
import { useSSEStatus } from '~/composables/useSSE'
import logoImage from '~/assets/img/logo-d-a.webp'
import ThemeToggle from './ThemeToggle.vue'

defineOptions({ name: 'AppSidebar' })

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
    iconColor: 'text-emerald-500',
  },
  { id: 'tokens', label: 'Tokens', icon: Key, path: '/tokens', iconColor: 'text-amber-500' },
  { id: 'nodes', label: 'Nodes', icon: Globe, path: '/nodes', iconColor: 'text-sky-500' },
  {
    id: 'inbounds',
    label: 'Inbounds',
    icon: Server,
    path: '/inbounds',
    iconColor: 'text-violet-500',
  },
  {
    id: 'settings',
    label: 'Settings',
    icon: Settings,
    path: '/settings',
    iconColor: 'text-rose-500',
  },
]

const activeItem = computed(() => {
  const activeNav = navItems.find((item) => route.path === item.path)
  return activeNav?.id || 'dashboard'
})

const handleNavClick = (path: string) => {
  sidebar.closeMobile()
  navigateTo(path)
}

const handleLogout = () => {
  auth.clearToken()
  sidebar.closeMobile()
  navigateTo('/login')
}

const handleCloseMobile = () => {
  sidebar.closeMobile()
}
</script>

<template>
  <!-- Mobile drawer -->
  <Teleport to="body">
    <div
      class="fixed inset-0 z-40 bg-black/60 transition-opacity duration-300 md:hidden"
      :class="
        sidebar.isMobileOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
      "
      @click="sidebar.closeMobile()"
    />
    <aside
      class="fixed left-0 top-0 z-50 h-screen w-full flex flex-col border-r bg-background transition-transform duration-300 ease-in-out md:hidden"
      :class="sidebar.isMobileOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div class="flex items-center justify-between border-b border-border p-4">
        <div class="flex items-center gap-3">
          <img :src="logoImage" alt="Outless Logo" class="h-10 w-10 flex-shrink-0" />
          <span class="font-bold text-lg text-foreground">
            Outless
            <ClientOnly>
              <span
                class="ml-2 inline-block h-2.5 w-2.5 rounded-full"
                :class="isBackendAvailable ? 'bg-emerald-500' : 'bg-red-500'"
              />
              <template #fallback>
                <span class="ml-2 inline-block h-2.5 w-2.5 rounded-full bg-muted-foreground/40" />
              </template>
            </ClientOnly>
          </span>
        </div>
        <button
          class="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
          @click="handleCloseMobile()"
        >
          <X class="h-5 w-5" />
        </button>
      </div>
      <nav class="flex-1 space-y-2 overflow-y-auto p-4">
        <button
          v-for="item in navItems"
          :key="item.id"
          class="group w-full flex items-center gap-3 rounded-lg p-3 transition-colors"
          :class="
            activeItem === item.id
              ? 'bg-primary/10 text-primary dark:bg-primary/20'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground'
          "
          @click="handleNavClick(item.path)"
        >
          <component
            :is="item.icon"
            :class="[
              item.iconColor,
              'h-5 w-5 flex-shrink-0 transition-transform duration-200 ease-out group-hover:scale-110 group-hover:rotate-3',
            ]"
          />
          <span class="font-medium">{{ item.label }}</span>
        </button>
      </nav>
      <div class="border-t border-border p-4">
        <button
          class="group w-full flex items-center gap-3 rounded-lg p-3 transition-colors text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
          @click="handleLogout"
        >
          <LogOut
            class="h-5 w-5 flex-shrink-0 transition-transform duration-200 ease-out group-hover:scale-110 group-hover:rotate-3"
          />
          <span class="font-medium">Logout</span>
        </button>
      </div>
    </aside>
  </Teleport>

  <!-- Desktop sidebar -->
  <aside
    class="hidden h-screen flex-col border-r bg-background transition-all duration-300 md:flex"
    :class="sidebar.isExpanded ? 'w-72' : 'w-20'"
  >
    <!-- Logo Section -->
    <div class="border-b border-border p-4">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <img :src="logoImage" alt="Outless Logo" class="h-12 w-12 flex-shrink-0" />
          <span v-if="sidebar.isExpanded" class="font-bold text-lg text-foreground">
            Outless
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
    <nav class="flex-1 space-y-2 overflow-y-auto p-4">
      <template v-for="item in navItems" :key="item.id">
        <div class="group relative">
          <button
            class="group flex w-full items-center justify-between rounded-lg p-3 transition-colors"
            :class="
              activeItem === item.id
                ? 'bg-primary/10 text-primary dark:bg-primary/20'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground'
            "
            @click="handleNavClick(item.path)"
          >
            <div class="flex items-center gap-3">
              <component
                :is="item.icon"
                :class="[
                  item.iconColor,
                  'h-5 w-5 flex-shrink-0 transition-transform duration-200 ease-out group-hover:scale-110 group-hover:rotate-3',
                ]"
              />
              <span v-if="sidebar.isExpanded" class="font-medium">{{ item.label }}</span>
            </div>
          </button>
        </div>
      </template>
    </nav>

    <!-- Logout Button -->
    <div class="border-t border-border p-4">
      <button
        class="group flex w-full items-center gap-3 rounded-lg p-3 transition-colors text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
        @click="handleLogout"
      >
        <LogOut
          class="h-5 w-5 flex-shrink-0 transition-transform duration-200 ease-out group-hover:scale-110 group-hover:rotate-3"
        />
        <span v-if="sidebar.isExpanded" class="font-medium">Logout</span>
      </button>
    </div>
  </aside>
</template>
