<script setup lang="ts">
import { computed, watch } from "vue";
import { useColorMode } from "@vueuse/core";
import Sidebar from "~/components/Sidebar.vue";
import { Toaster } from "vue-sonner";
import { useAuth } from "~/composables/useAuth";

const colorMode = useColorMode({ emitAuto: true });
const auth = useAuth();

const toasterTheme = computed<"light" | "dark">(() => {
  // 'auto' means system preference, check actual dark class on html
  if (colorMode.value === "dark") return "dark";
  if (colorMode.value === "light") return "light";
  // auto - check what actually rendered on client, fallback to light on server
  if (typeof document !== "undefined") {
    return document.documentElement.classList.contains("dark")
      ? "dark"
      : "light";
  }
  return "light";
});

watch(
  auth.isAuthenticated,
  (isAuthenticated) => {
    console.log("[LAYOUT] isAuthenticated changed:", isAuthenticated);
    if (!isAuthenticated && import.meta.client) {
      console.log("[LAYOUT] Not authenticated, redirecting to /login");
      navigateTo("/login");
    }
  },
  { immediate: true },
);
</script>

<template>
  <div>
    <div class="flex min-h-screen bg-background">
      <div class="sticky top-0 h-screen">
        <Sidebar />
      </div>
      <div class="flex-1 flex flex-col overflow-hidden">
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
