<script setup lang="ts">
definePageMeta({
  layout: false,
  middleware: [
    () => navigateTo('/tokens', { redirectCode: 301 }),
  ],
})
</script>

<template>
  <div />
</template>
