<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("sync.syncFolder") }}</h2>
    </div>

    <div class="card-content">
      <p class="sync-path">{{ path }}</p>

      <p v-if="loading">{{ t("files.loading") }}</p>

      <template v-else>
        <template v-if="!addingDevice && devices.length > 0">
          <p class="sync-label">{{ t("sync.chooseDevices") }}</p>
          <label v-for="device in devices" :key="device.id" class="sync-row">
            <input type="checkbox" :value="device.id" v-model="selected" />
            <span class="sync-dot" :class="{ on: device.connected }"></span>
            {{ device.name }}
            <small>{{ device.address }}</small>
          </label>
          <p>
            <a class="sync-link" @click="addingDevice = true">{{
              t("sync.addNewDevice")
            }}</a>
          </p>

          <p class="sync-label">{{ t("sync.mode") }}</p>
          <label class="sync-row">
            <input type="radio" value="oneway" v-model="mode" />
            {{ t("sync.modeOneWayLong") }}
          </label>
          <label class="sync-row">
            <input type="radio" value="twoway" v-model="mode" />
            {{ t("sync.modeTwoWayLong") }}
          </label>

          <label class="sync-row">
            <input type="checkbox" v-model="versioning" />
            {{ t("sync.versioning") }}
          </label>

          <p class="sync-note">{{ t("sync.acceptNote") }}</p>
        </template>

        <template v-else>
          <p v-if="devices.length === 0">{{ t("sync.noDevicesYet") }}</p>
          <p class="sync-label">
            {{ t("sync.thisDeviceId") }}
            <code>{{ deviceID }}</code>
          </p>
          <sync-device-form @added="deviceAdded" />
          <p v-if="devices.length > 0">
            <a class="sync-link" @click="addingDevice = false">{{
              t("buttons.cancel")
            }}</a>
          </p>
        </template>
      </template>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers"
        :aria-label="t('buttons.cancel')"
        :title="t('buttons.cancel')"
      >
        {{ t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat"
        :disabled="!canStart"
        @click="start"
        :aria-label="t('sync.start')"
        :title="t('sync.start')"
      >
        {{ t("sync.start") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { sync as api } from "@/api";
import { useLayoutStore } from "@/stores/layout";
import SyncDeviceForm from "@/components/settings/SyncDeviceForm.vue";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const layoutStore = useLayoutStore();

const path = computed<string>(() => layoutStore.currentPrompt?.props?.path);

const loading = ref(true);
const devices = ref<SyncDevice[]>([]);
const deviceID = ref("");
const addingDevice = ref(false);
const selected = ref<string[]>([]);
const mode = ref<"oneway" | "twoway">("oneway");
const versioning = ref(true);

const canStart = computed(
  () => !loading.value && !addingDevice.value && selected.value.length > 0
);

const load = async () => {
  loading.value = true;
  try {
    const status = await api.status();
    devices.value = status.devices;
    deviceID.value = status.deviceID;
    addingDevice.value = status.devices.length === 0;
  } catch (e) {
    $showError(e as Error);
    layoutStore.closeHovers();
  } finally {
    loading.value = false;
  }
};

const deviceAdded = async () => {
  await load();
  addingDevice.value = false;
};

const start = async () => {
  try {
    await api.createFolder({
      path: path.value,
      deviceIDs: selected.value,
      mode: mode.value,
      versioning: versioning.value,
    });
    $showSuccess(t("sync.started"));
    layoutStore.closeHovers();
  } catch (e) {
    $showError(e as Error);
  }
};

// The prompt is kept alive between uses, so reset and reload each time it is
// shown (onActivated also runs on the first mount).
onActivated(() => {
  selected.value = [];
  mode.value = "oneway";
  versioning.value = true;
  load();
});
</script>

<style scoped>
.sync-path {
  margin-top: 0;
  word-break: break-all;
  opacity: 0.7;
}

.sync-label {
  margin: 1em 0 0.3em;
  font-weight: bold;
}

.sync-label code {
  display: block;
  font-weight: normal;
  word-break: break-all;
}

.sync-row {
  display: block;
  margin: 0.3em 0;
}

.sync-row small {
  margin-left: 0.5em;
  opacity: 0.6;
}

.sync-dot {
  display: inline-block;
  width: 0.6em;
  height: 0.6em;
  margin: 0 0.3em;
  border-radius: 50%;
  background: #9e9e9e;
}

.sync-dot.on {
  background: #43a047;
}

.sync-note {
  margin-top: 1em;
  font-size: 0.9em;
  opacity: 0.75;
}

.sync-link {
  cursor: pointer;
  color: var(--blue);
}
</style>
