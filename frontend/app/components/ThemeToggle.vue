<script setup lang="ts">
import { Moon, Sun } from 'lucide-vue-next'

const { toggleTheme } = useTheme()
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-10 w-10"
    @click="toggleTheme"
  >
    <Sun class="h-5 w-5 dark:hidden" />
    <Moon class="h-5 w-5 hidden dark:block" />
  </button>
</template>
