<script setup lang="ts">
import { computed, ref } from "vue";
import { GripVertical } from "lucide-vue-next";

const props = defineProps<{
  modelValue: string;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const TOKEN_PREVIEW: Record<string, string> = {
  "vless.group": "Europe",
  "vless.name": "NL-VPS-01",
  "vless.host": "1.2.3.4",
  "vless.port": "443",
  "vless.sni": "www.google.com",
  "vless.security": "reality",
  "vless.encryption": "none",
  "vless.flow": "xtls-rprx-vision",
  "vless.fp": "chrome",
  "vless.user": "user-123",
};

const TOKENS = [
  { label: "Group", value: "{{vless.group}}", key: "vless.group" },
  { label: "Node Name", value: "{{vless.name}}", key: "vless.name" },
  { label: "Host", value: "{{vless.host}}", key: "vless.host" },
  { label: "Port", value: "{{vless.port}}", key: "vless.port" },
  { label: "SNI", value: "{{vless.sni}}", key: "vless.sni" },
  { label: "Security", value: "{{vless.security}}", key: "vless.security" },
  { label: "Flow", value: "{{vless.flow}}", key: "vless.flow" },
  { label: "Fingerprint", value: "{{vless.fp}}", key: "vless.fp" },
  { label: "User", value: "{{vless.user}}", key: "vless.user" },
];

const textareaRef = ref<HTMLTextAreaElement | null>(null);
const isDraggingOver = ref(false);

function renderPreview(tmpl: string): string {
  const re = /\{\{([a-zA-Z0-9_.]+)(?:\|([^{} ]+))?\}\}/g;
  return tmpl.replace(
    re,
    (_match, key: string, fallback: string | undefined) => {
      if (TOKEN_PREVIEW[key]) return TOKEN_PREVIEW[key];
      if (fallback) {
        if (fallback.startsWith('"') && fallback.endsWith('"')) {
          return fallback.slice(1, -1);
        }
        return TOKEN_PREVIEW[fallback] || `{{${key}}}`;
      }
      return `{{${key}}}`;
    },
  );
}

const preview = computed(() => renderPreview(props.modelValue));

function updateValue(val: string) {
  emit("update:modelValue", val);
}

function insertToken(token: string) {
  const el = textareaRef.value;
  if (!el) {
    updateValue(props.modelValue + token);
    return;
  }
  const start = el.selectionStart ?? props.modelValue.length;
  const end = el.selectionEnd ?? props.modelValue.length;
  const before = props.modelValue.slice(0, start);
  const after = props.modelValue.slice(end);
  const next = before + token + after;
  updateValue(next);
  requestAnimationFrame(() => {
    el.focus();
    const pos = start + token.length;
    el.selectionStart = pos;
    el.selectionEnd = pos;
  });
}

function handleDragStart(e: DragEvent, token: string) {
  e.dataTransfer?.setData("text/plain", token);
  e.dataTransfer!.effectAllowed = "copy";
}

function handleDragOver(e: DragEvent) {
  e.preventDefault();
  e.dataTransfer!.dropEffect = "copy";
  isDraggingOver.value = true;
}

function handleDragLeave() {
  isDraggingOver.value = false;
}

function handleDrop(e: DragEvent) {
  e.preventDefault();
  isDraggingOver.value = false;
  const token = e.dataTransfer?.getData("text/plain");
  if (!token || !textareaRef.value) return;

  const el = textareaRef.value;
  const rect = el.getBoundingClientRect();
  const lineHeight = parseFloat(getComputedStyle(el).lineHeight) || 20;
  const paddingTop = parseFloat(getComputedStyle(el).paddingTop) || 8;
  const paddingLeft = parseFloat(getComputedStyle(el).paddingLeft) || 12;

  const relativeY = e.clientY - rect.top - paddingTop;
  const relativeX = e.clientX - rect.left - paddingLeft;
  const approxLine = Math.max(0, Math.floor(relativeY / lineHeight));

  const lines = props.modelValue.split("\n");
  let charIndex = 0;
  for (let i = 0; i < Math.min(approxLine, lines.length); i++) {
    charIndex += (lines[i] || "").length + 1; // +1 for newline
  }

  const targetLine = lines[Math.min(approxLine, lines.length - 1)] || "";
  const approxCol = Math.min(
    Math.max(0, Math.round(relativeX / 7.5)),
    targetLine.length,
  );
  const insertPos = charIndex + approxCol;

  const before = props.modelValue.slice(0, insertPos);
  const after = props.modelValue.slice(insertPos);
  const next = before + token + after;
  updateValue(next);
  requestAnimationFrame(() => {
    el.focus();
    const pos = insertPos + token.length;
    el.selectionStart = pos;
    el.selectionEnd = pos;
  });
}

function onClickToken(token: string) {
  insertToken(token);
}
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-1.5">
      <label class="text-sm font-medium">Available Variables</label>
      <div class="flex flex-wrap gap-1.5">
        <button
          v-for="t in TOKENS"
          :key="t.value"
          draggable="true"
          type="button"
          class="inline-flex cursor-grab select-none items-center gap-1 rounded-md border bg-muted px-2 py-1 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground active:cursor-grabbing"
          @dragstart="handleDragStart($event, t.value)"
          @click="onClickToken(t.value)"
        >
          <GripVertical class="h-3 w-3 text-muted-foreground" />
          {{ t.label }}
        </button>
      </div>
    </div>

    <div class="space-y-1.5">
      <label class="text-sm font-medium">Template</label>
      <textarea
        ref="textareaRef"
        :value="modelValue"
        class="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
        placeholder="{{vless.country}} | {{vless.group}}"
        @dragover="handleDragOver"
        @dragleave="handleDragLeave"
        @drop="handleDrop"
        @input="
          ($event) => updateValue(($event.target as HTMLTextAreaElement).value)
        "
      />
      <p class="text-xs text-muted-foreground">
        Click a variable above to insert it. You can also drag variables into
        the template field.
      </p>
    </div>

    <div class="space-y-1.5">
      <label class="text-sm font-medium">Preview</label>
      <div
        class="rounded-md border border-border/70 bg-muted/50 px-3 py-2 text-sm text-foreground/80"
      >
        <span v-if="preview" class="font-medium">{{ preview }}</span>
        <span v-else class="text-muted-foreground italic"
          >Start building your template to see the preview</span
        >
      </div>
    </div>
  </div>
</template>
