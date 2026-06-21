<script setup lang="ts">
import { ref, computed } from "vue";
import { Plus, Copy, Check } from "lucide-vue-next";
import { toast } from "vue-sonner";
import type { Inbound, CreateInbound } from "~/utils/schemas/inbound";
import {
  useInbounds,
  useCreateInbound,
  useUpdateInbound,
  useDeleteInbound,
} from "~/composables/inbounds/useInbounds";
import UiButton from "~/components/ui/button/button.vue";
import UiInput from "~/components/ui/input/input.vue";
import UiSelect from "~/components/ui/select/select.vue";
import UiCard from "~/components/ui/card/card.vue";
import CardHeader from "~/components/ui/card/CardHeader.vue";
import CardTitle from "~/components/ui/card/CardTitle.vue";
import CardContent from "~/components/ui/card/CardContent.vue";
import CardFooter from "~/components/ui/card/CardFooter.vue";

const { data: inbounds, isLoading } = useInbounds();
const createMutation = useCreateInbound();
const updateMutation = useUpdateInbound();
const deleteMutation = useDeleteInbound();

const showCreateDialog = ref(false);
const showEditDialog = ref(false);
const selectedInbound = ref<Inbound | null>(null);
const copiedKeyId = ref<string | null>(null);
const copiedUrlId = ref<string | null>(null);

type InboundForm = Omit<CreateInbound, "port"> & { port: string | number };

const FINGERPRINT_OPTIONS = [
  { label: "Random", value: "random" },
  { label: "Randomized", value: "randomized" },
  { label: "Chrome", value: "chrome" },
  { label: "Firefox", value: "firefox" },
  { label: "Safari", value: "safari" },
  { label: "Edge", value: "edge" },
  { label: "iOS", value: "ios" },
  { label: "Android", value: "android" },
  { label: "360", value: "360" },
  { label: "QQ", value: "qq" },
];

const ADDRESS_OPTIONS = [
  { label: "0.0.0.0 (All IPv4)", value: "0.0.0.0" },
  { label: "127.0.0.1 (Local)", value: "127.0.0.1" },
];

const TEMPLATE_OPTIONS = [
  { label: "Country | Group", value: "{{vless.country}} | {{vless.group}}" },
  { label: "Country - Group", value: "{{vless.country}} - {{vless.group}}" },
  { label: "Node Name", value: "{{vless.name}}" },
  { label: "Custom", value: "__custom__" },
];

function generateShortId(): string {
  const arr = new Uint8Array(8);
  crypto.getRandomValues(arr);
  return Array.from(arr, (b) => b.toString(16).padStart(2, "0")).join("");
}

const form = ref<InboundForm>({
  name: "",
  address: "0.0.0.0",
  port: 443,
  sni: "",
  handshake: "",
  private_key: "",
  short_id: generateShortId(),
  fingerprint: "random",
  url_host: "",
  name_template: "",
});

const isCreateSubmitting = ref(false);
const isEditSubmitting = ref(false);

const addressSelectValue = computed({
  get: () =>
    ADDRESS_OPTIONS.some((o) => o.value === form.value.address)
      ? form.value.address
      : "__custom__",
  set: (val: string) => {
    if (val !== "__custom__") {
      form.value.address = val;
    }
  },
});

const isCustomAddress = computed(
  () => addressSelectValue.value === "__custom__",
);

const templateSelectValue = computed({
  get: () =>
    TEMPLATE_OPTIONS.some((o) => o.value === form.value.name_template)
      ? form.value.name_template
      : "__custom__",
  set: (val: string) => {
    if (val !== "__custom__") {
      form.value.name_template = val;
    }
  },
});

const isCustomTemplate = computed(
  () => templateSelectValue.value === "__custom__",
);

function resetForm() {
  form.value = {
    name: "",
    address: "0.0.0.0",
    port: 443,
    sni: "",
    handshake: "",
    private_key: "",
    short_id: generateShortId(),
    fingerprint: "random",
    url_host: "",
    name_template: "",
  };
}

function fillForm(inbound: Inbound) {
  form.value = {
    name: inbound.name,
    address: inbound.address,
    port: inbound.port,
    sni: inbound.sni,
    handshake: inbound.handshake,
    private_key: "",
    short_id: inbound.short_id,
    fingerprint: inbound.fingerprint,
    url_host: inbound.url_host,
    name_template: inbound.name_template,
  };
}

function openCreateDialog() {
  createMutation.reset();
  isCreateSubmitting.value = false;
  resetForm();
  showCreateDialog.value = true;
}

function closeCreateDialog() {
  showCreateDialog.value = false;
  resetForm();
}

function openEditDialog(inbound: Inbound) {
  updateMutation.reset();
  isEditSubmitting.value = false;
  selectedInbound.value = inbound;
  fillForm(inbound);
  showEditDialog.value = true;
}

function closeEditDialog() {
  showEditDialog.value = false;
  selectedInbound.value = null;
  resetForm();
}

function buildPayload(): CreateInbound {
  const port = parseInt(String(form.value.port), 10);
  return {
    ...form.value,
    port: Number.isNaN(port) ? 443 : port,
  };
}

function handleCreate() {
  if (!form.value.name.trim() || isCreateSubmitting.value) return;
  isCreateSubmitting.value = true;
  createMutation.mutate(buildPayload(), {
    onSuccess: () => {
      showCreateDialog.value = false;
      resetForm();
      toast.success("Inbound created");
    },
    onSettled: () => {
      isCreateSubmitting.value = false;
    },
  });
}

function handleUpdate() {
  if (
    !selectedInbound.value ||
    !form.value.name.trim() ||
    isEditSubmitting.value
  )
    return;
  isEditSubmitting.value = true;
  updateMutation.mutate(
    { id: selectedInbound.value.id, data: buildPayload() },
    {
      onSuccess: () => {
        showEditDialog.value = false;
        selectedInbound.value = null;
        resetForm();
        toast.success("Inbound updated");
      },
      onSettled: () => {
        isEditSubmitting.value = false;
      },
    },
  );
}

function handleDelete(inbound: Inbound) {
  if (!confirm(`Are you sure you want to delete inbound "${inbound.name}"?`))
    return;
  deleteMutation.mutate(inbound.id, {
    onSuccess: () => toast.success("Inbound deleted"),
  });
}

function copyPublicKey(inbound: Inbound) {
  navigator.clipboard.writeText(inbound.public_key);
  copiedKeyId.value = inbound.id;
  toast.success("Public key copied");
  setTimeout(() => (copiedKeyId.value = null), 1500);
}

function subscriptionUrl(inbound: Inbound) {
  const base = window.location.origin;
  return `${base}/v1/sub/{token}?inbound_id=${inbound.id}`;
}

function copySubscriptionUrl(inbound: Inbound) {
  navigator.clipboard.writeText(subscriptionUrl(inbound));
  copiedUrlId.value = inbound.id;
  toast.success("Subscription URL copied");
  setTimeout(() => (copiedUrlId.value = null), 1500);
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex justify-end items-center">
      <UiButton @click="openCreateDialog">
        <Plus class="h-4 w-4 mr-2" />
        Create Inbound
      </UiButton>
    </div>

    <div v-if="isLoading" class="text-center text-muted-foreground py-8">
      Loading inbounds...
    </div>
    <div
      v-else-if="!inbounds || inbounds.length === 0"
      class="text-center text-muted-foreground py-8"
    >
      No inbounds configured
    </div>

    <UiCard v-for="inbound in inbounds" :key="inbound.id" class="p-4">
      <CardContent class="p-0">
        <div
          class="flex flex-col md:flex-row md:items-center justify-between gap-4"
        >
          <div class="space-y-1">
            <h3 class="font-semibold text-lg">{{ inbound.name }}</h3>
            <p class="text-muted-foreground text-sm">
              {{ inbound.address }}:{{ inbound.port }} · SNI:
              {{ inbound.sni || "-" }}
            </p>
            <p class="text-muted-foreground text-sm">
              URL Host: {{ inbound.url_host || "-" }} · Fingerprint:
              {{ inbound.fingerprint }}
            </p>
            <p class="text-muted-foreground text-sm">
              Public Key: {{ inbound.public_key.slice(0, 16) }}...{{
                inbound.public_key.slice(-8)
              }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <UiButton
              variant="outline"
              size="sm"
              @click="copyPublicKey(inbound)"
            >
              <component
                :is="copiedKeyId === inbound.id ? Check : Copy"
                class="h-4 w-4 mr-1"
              />
              Copy Key
            </UiButton>
            <UiButton
              variant="outline"
              size="sm"
              @click="copySubscriptionUrl(inbound)"
            >
              <component
                :is="copiedUrlId === inbound.id ? Check : Copy"
                class="h-4 w-4 mr-1"
              />
              Copy Sub URL
            </UiButton>
            <UiButton
              variant="outline"
              size="sm"
              @click="openEditDialog(inbound)"
            >
              Edit
            </UiButton>
            <UiButton
              variant="destructive"
              size="sm"
              :disabled="deleteMutation.isPending"
              @click="handleDelete(inbound)"
            >
              Delete
            </UiButton>
          </div>
        </div>
      </CardContent>
    </UiCard>

    <!-- Create Dialog -->
    <div
      v-if="showCreateDialog"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
    >
      <UiCard class="w-full max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <CardHeader>
          <CardTitle>Create Inbound</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Name</label>
              <UiInput v-model="form.name" placeholder="EU Entry" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Listen Address</label>
              <UiSelect
                v-model="addressSelectValue"
                :options="ADDRESS_OPTIONS"
              />
              <UiInput
                v-if="isCustomAddress"
                v-model="form.address"
                placeholder="192.168.1.10"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Port</label>
              <UiInput v-model="form.port" type="number" placeholder="443" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">SNI</label>
              <UiInput v-model="form.sni" placeholder="www.google.com" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Handshake Server</label>
              <UiInput v-model="form.handshake" placeholder="www.google.com" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium"
                >Private Key (optional, generates if empty)</label
              >
              <UiInput
                v-model="form.private_key"
                placeholder="base64 private key"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Short ID</label>
              <div class="flex gap-2">
                <UiInput
                  v-model="form.short_id"
                  placeholder=""
                  class="flex-1"
                />
                <UiButton
                  type="button"
                  variant="outline"
                  class="h-10"
                  @click="form.short_id = generateShortId()"
                >
                  Generate
                </UiButton>
              </div>
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Fingerprint</label>
              <UiSelect
                v-model="form.fingerprint"
                :options="FINGERPRINT_OPTIONS"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">URL Host</label>
              <UiInput v-model="form.url_host" placeholder="example.com" />
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-sm font-medium">Name Template</label>
              <UiSelect
                v-model="templateSelectValue"
                :options="TEMPLATE_OPTIONS"
              />
              <UiInput
                v-if="isCustomTemplate"
                v-model="form.name_template"
                placeholder="{{vless.country}} | {{vless.group}}"
              />
            </div>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeCreateDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!form.name.trim() || isCreateSubmitting"
            @click="handleCreate"
          >
            {{ isCreateSubmitting ? "Creating..." : "Create" }}
          </UiButton>
        </CardFooter>
      </UiCard>
    </div>

    <!-- Edit Dialog -->
    <div
      v-if="showEditDialog"
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
    >
      <UiCard class="w-full max-w-2xl max-h-[90vh] overflow-y-auto p-6">
        <CardHeader>
          <CardTitle>Edit Inbound</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="text-sm font-medium">Name</label>
              <UiInput v-model="form.name" placeholder="EU Entry" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Listen Address</label>
              <UiSelect
                v-model="addressSelectValue"
                :options="ADDRESS_OPTIONS"
              />
              <UiInput
                v-if="isCustomAddress"
                v-model="form.address"
                placeholder="192.168.1.10"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Port</label>
              <UiInput v-model="form.port" type="number" placeholder="443" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">SNI</label>
              <UiInput v-model="form.sni" placeholder="www.google.com" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Handshake Server</label>
              <UiInput v-model="form.handshake" placeholder="www.google.com" />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium"
                >Private Key (leave blank to keep current)</label
              >
              <UiInput
                v-model="form.private_key"
                placeholder="base64 private key"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Short ID</label>
              <div class="flex gap-2">
                <UiInput
                  v-model="form.short_id"
                  placeholder=""
                  class="flex-1"
                />
                <UiButton
                  type="button"
                  variant="outline"
                  class="h-10"
                  @click="form.short_id = generateShortId()"
                >
                  Generate
                </UiButton>
              </div>
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">Fingerprint</label>
              <UiSelect
                v-model="form.fingerprint"
                :options="FINGERPRINT_OPTIONS"
              />
            </div>
            <div class="space-y-2">
              <label class="text-sm font-medium">URL Host</label>
              <UiInput v-model="form.url_host" placeholder="example.com" />
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-sm font-medium">Name Template</label>
              <UiSelect
                v-model="templateSelectValue"
                :options="TEMPLATE_OPTIONS"
              />
              <UiInput
                v-if="isCustomTemplate"
                v-model="form.name_template"
                placeholder="{{vless.country}} | {{vless.group}}"
              />
            </div>
          </div>
        </CardContent>
        <CardFooter class="flex justify-end gap-2">
          <UiButton variant="outline" @click="closeEditDialog">
            Cancel
          </UiButton>
          <UiButton
            :disabled="!form.name.trim() || isEditSubmitting"
            @click="handleUpdate"
          >
            {{ isEditSubmitting ? "Updating..." : "Update" }}
          </UiButton>
        </CardFooter>
      </UiCard>
    </div>
  </div>
</template>
