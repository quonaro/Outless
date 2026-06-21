<script setup lang="ts">
import { ref, computed } from "vue";
import { Zap, Copy, Check } from "lucide-vue-next";
import UiCard from "~/components/ui/card/card.vue";
import CardContent from "~/components/ui/card/CardContent.vue";
import UiButton from "~/components/ui/button/button.vue";
import { useInbounds } from "~/composables/inbounds/useInbounds";

const { data: inbounds } = useInbounds();

const activeInbound = computed(() => {
  if (!inbounds.value || inbounds.value.length === 0) return null;
  return (
    inbounds.value.find((i) => i.enable_auto_self_node) ?? inbounds.value[0]
  );
});

const copied = ref(false);

const nodeName = computed(
  () => activeInbound.value?.auto_self_node_name || "Direct Exit",
);

const publicKey = computed(() => activeInbound.value?.public_key || "");
const shortId = computed(() => activeInbound.value?.short_id || "");
const sni = computed(() => activeInbound.value?.sni || "");
const host = computed(() => activeInbound.value?.url_host || "");
const port = computed(() => activeInbound.value?.port || 443);

// Generate the VLESS URL for the auto self-node
// This is a template - the actual UUID is determined per-token
const vlessUrl = computed(() => {
  if (!activeInbound.value) return "";
  const fp = activeInbound.value?.fingerprint || "chrome";
  const params = new URLSearchParams({
    encryption: "none",
    security: "reality",
    type: "tcp",
    flow: "xtls-rprx-vision",
    sni: sni.value,
    fp: fp,
    ...(publicKey.value ? { pbk: publicKey.value } : {}),
    sid: shortId.value,
  });

  return `vless://<uuid>@${host.value}:${port.value}?${params.toString()}#${encodeURIComponent(nodeName.value)}`;
});

async function copyUrl() {
  await navigator.clipboard.writeText(vlessUrl.value);
  copied.value = true;
  setTimeout(() => (copied.value = false), 1500);
}
</script>

<template>
  <UiCard
    class="px-3 py-2 border-emerald-500/30 bg-emerald-50/50 dark:bg-emerald-950/20"
  >
    <CardContent class="p-0">
      <div class="flex items-center justify-between gap-2">
        <div class="max-w-[52%] min-w-0">
          <div class="flex items-center gap-2">
            <Zap class="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            <p
              class="truncate text-sm font-medium text-emerald-700 dark:text-emerald-300"
            >
              {{ nodeName }}
            </p>
            <span
              class="inline-flex shrink-0 items-center rounded-full bg-emerald-100 dark:bg-emerald-900/50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:text-emerald-300"
            >
              DIRECT
            </span>
          </div>
          <p class="text-xs text-muted-foreground">
            Auto-generated · Direct exit through server
          </p>
        </div>
        <div class="flex shrink-0 flex-nowrap gap-1">
          <UiButton
            variant="outline"
            size="sm"
            class="whitespace-nowrap border-emerald-500/30 hover:bg-emerald-100 dark:hover:bg-emerald-900/30"
            @click="copyUrl"
          >
            <component :is="copied ? Check : Copy" class="h-3.5 w-3.5 mr-1.5" />
            {{ copied ? "Copied" : "Copy" }}
          </UiButton>
        </div>
      </div>
    </CardContent>
  </UiCard>
</template>
