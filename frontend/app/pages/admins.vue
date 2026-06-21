<script setup lang="ts">
definePageMeta({
  layout: false,
  middleware: [
    () => navigateTo('/dashboard', { redirectCode: 301 }),
  ],
})
</script>

<template>
  <div />
</template>
