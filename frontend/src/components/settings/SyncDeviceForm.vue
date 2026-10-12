<template>
  <form class="sync-device-form" @submit.prevent="submit">
    <label>
      <span>{{ t("sync.deviceId") }}</span>
      <input
        class="input input--block"
        type="text"
        v-model.trim="deviceID"
        placeholder="XXXXXXX-XXXXXXX-XXXXXXX-XXXXXXX-XXXXXXX-XXXXXXX-XXXXXXX-XXXXXXX"
        spellcheck="false"
        :disabled="lockId"
        required
      />
    </label>
    <label>
      <span>{{ t("sync.deviceName") }}</span>
      <input
        class="input input--block"
        type="text"
        v-model.trim="name"
        maxlength="100"
        required
      />
    </label>
    <label>
      <span>{{ t("sync.deviceAddress") }}</span>
      <input
        class="input input--block"
        type="text"
        v-model.trim="address"
        placeholder="100.64.0.30"
        spellcheck="false"
      />
      <small>{{
        t("sync.deviceAddressHelp", { port: defaultPort ?? "22000" })
      }}</small>
    </label>
    <button class="button" type="submit" :disabled="busy">
      {{ submitLabel ?? t("sync.addDevice") }}
    </button>
  </form>
</template>

<script setup lang="ts">
import { inject, ref } from "vue";
import { useI18n } from "vue-i18n";
import { sync as api } from "@/api";

const props = defineProps<{
  initial?: { deviceID?: string; name?: string; address?: string };
  lockId?: boolean;
  defaultPort?: string;
  submitLabel?: string;
}>();

const emit = defineEmits<{ (e: "added"): void }>();

const $showError = inject<IToastError>("$showError")!;
const { t } = useI18n();

const deviceID = ref(props.initial?.deviceID ?? "");
const name = ref(props.initial?.name ?? "");
const address = ref(props.initial?.address ?? "");
const busy = ref(false);

const submit = async () => {
  busy.value = true;
  try {
    await api.addDevice({
      deviceID: deviceID.value,
      name: name.value,
      address: address.value,
    });
    deviceID.value = "";
    name.value = "";
    address.value = "";
    emit("added");
  } catch (e) {
    $showError(e as Error);
  } finally {
    busy.value = false;
  }
};
</script>

<style scoped>
.sync-device-form label {
  display: block;
  margin-bottom: 0.8em;
}

.sync-device-form label > span {
  display: block;
  margin-bottom: 0.2em;
  font-size: 0.9em;
  opacity: 0.8;
}

.sync-device-form small {
  display: block;
  margin-top: 0.2em;
  opacity: 0.7;
}
</style>
