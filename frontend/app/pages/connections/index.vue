<script setup lang="ts">
definePageMeta({
  layout: 'default',
  middleware: [() => navigateTo('/connections/active', { redirectCode: 301 })],
})
</script>
