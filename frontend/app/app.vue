<script setup lang="ts">
const colorMode = useNuxtApp().$colorMode

useHead(() => ({
  titleTemplate: '%s | Outless',
  title: 'Outless',
  meta: [
    {
      name: 'theme-color',
      content: colorMode.value === 'dark' ? '#0a0a0a' : '#ffffff',
    },
  ],
}))

useTheme()
</script>

<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <ConfirmDialog />
</template>
