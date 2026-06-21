<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'

const { isDark, toggleTheme } = useTheme()
</script>

<template>
  <ClientOnly>
    <button
      type="button"
      @click="toggleTheme"
      class="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-10 w-10"
    >
      <Moon v-if="isDark" class="h-5 w-5" />
      <Sun v-else class="h-5 w-5" />
    </button>
  </ClientOnly>
</template>
