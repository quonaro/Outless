<script setup lang="ts">
definePageMeta({
  layout: false,
  middleware: [
    () => navigateTo('/nodes', { redirectCode: 301 }),
  ],
})
</script>

<template>
  <div />
</template>
