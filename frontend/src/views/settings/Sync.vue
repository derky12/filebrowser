<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="status">
    <div class="column sync-column">
      <div class="sync-stack">
        <div class="card">
          <div class="card-title">
            <h2>{{ t("sync.thisDevice") }}</h2>
          </div>
          <div class="card-content">
            <p>{{ t("sync.thisDeviceHelp") }}</p>
            <p class="sync-id">
              <code>{{ status.deviceID }}</code>
              <button
                class="action"
                :title="t('buttons.copyToClipboard')"
                :aria-label="t('buttons.copyToClipboard')"
                @click="copyId"
              >
                <i class="material-icons">content_paste</i>
              </button>
            </p>
            <p v-if="guiUrl">
              <a
                class="button"
                :href="guiUrl"
                target="_blank"
                rel="noopener noreferrer"
              >
                {{ t("sync.openGui") }}
              </a>
            </p>
          </div>
        </div>

        <div
          class="card"
          v-if="
            status.pendingDevices.length > 0 || status.pendingFolders.length > 0
          "
        >
          <div class="card-title">
            <h2>{{ t("sync.pending") }}</h2>
          </div>
          <div class="card-content">
            <div v-for="p in status.pendingDevices" :key="p.id" class="pending">
              <p>
                <strong>{{ t("sync.pendingDevice", { name: p.name }) }}</strong>
                <small>{{ p.id }}</small>
              </p>
              <template v-if="acceptingDevice === p.id">
                <sync-device-form
                  :initial="{
                    deviceID: p.id,
                    name: p.name,
                    address: hostOf(p.address),
                  }"
                  lockId
                  :defaultPort="status.defaultPort"
                  :submitLabel="t('sync.accept')"
                  @added="
                    acceptingDevice = null;
                    refresh();
                  "
                />
              </template>
              <template v-else>
                <button class="button" @click="acceptingDevice = p.id">
                  {{ t("sync.accept") }}
                </button>
                <button
                  class="button button--flat button--grey"
                  @click="dismissDevice(p)"
                >
                  {{ t("sync.dismiss") }}
                </button>
              </template>
            </div>

            <div
              v-for="p in status.pendingFolders"
              :key="p.id + p.deviceID"
              class="pending"
            >
              <p>
                <strong>{{
                  t("sync.pendingFolder", {
                    label: p.label,
                    device: p.deviceName || p.deviceID,
                  })
                }}</strong>
              </p>
              <template v-if="acceptingFolder === p.id + p.deviceID">
                <label class="field">
                  <span>{{ t("sync.localPath") }}</span>
                  <path-input v-model="acceptPath" />
                  <small>{{ t("sync.localPathHelp") }}</small>
                </label>
                <label class="field">
                  <span>{{ t("sync.mode") }}</span>
                  <select class="input input--block" v-model="acceptMode">
                    <option value="receive">
                      {{ t("sync.modeReceiveLong") }}
                    </option>
                    <option value="twoway">
                      {{ t("sync.modeTwoWayLong") }}
                    </option>
                  </select>
                </label>
                <button class="button" @click="acceptFolder(p)">
                  {{ t("sync.accept") }}
                </button>
                <button
                  class="button button--flat button--grey"
                  @click="acceptingFolder = null"
                >
                  {{ t("buttons.cancel") }}
                </button>
              </template>
              <template v-else>
                <button class="button" @click="startAccept(p)">
                  {{ t("sync.accept") }}
                </button>
                <button
                  class="button button--flat button--grey"
                  @click="dismissFolder(p)"
                >
                  {{ t("sync.dismiss") }}
                </button>
              </template>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-title">
            <h2>{{ t("sync.folders") }}</h2>
          </div>
          <div
            class="card-content"
            v-if="status.devices.some((d) => d.connected)"
          >
            <p class="sync-summary">
              {{ t("sync.currentSpeed") }}
              <span class="sync-speed">↓ {{ speed(totalRate.down) }}</span>
              <span class="sync-speed">↑ {{ speed(totalRate.up) }}</span>
            </p>
          </div>
          <div class="card-content full" v-if="status.folders.length > 0">
            <table>
              <tr>
                <th>{{ t("settings.path") }}</th>
                <th>{{ t("sync.mode") }}</th>
                <th>{{ t("sync.devices") }}</th>
                <th>{{ t("sync.state") }}</th>
                <th></th>
                <th></th>
                <th></th>
              </tr>
              <tr v-for="f in status.folders" :key="f.id">
                <td>
                  <router-link v-if="!f.external" :to="'/files' + f.path">{{
                    f.path
                  }}</router-link>
                  <template v-else>{{ f.path }}</template>
                </td>
                <td>{{ modeLabel(f.mode) }}</td>
                <td>{{ remoteNames(f) }}</td>
                <td>
                  <span :class="['sync-state', stateClass(f)]">{{
                    stateLabel(f)
                  }}</span>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="t('sync.ignoreRules')"
                    :aria-label="t('sync.ignoreRules')"
                    @click="editIgnores(f)"
                  >
                    <i class="material-icons">filter_list</i>
                  </button>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="f.paused ? t('sync.resume') : t('sync.pause')"
                    :aria-label="f.paused ? t('sync.resume') : t('sync.pause')"
                    @click="togglePause(f)"
                  >
                    <i class="material-icons">{{
                      f.paused ? "play_arrow" : "pause"
                    }}</i>
                  </button>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="t('sync.stopSyncing')"
                    :aria-label="t('sync.stopSyncing')"
                    @click="removeFolder(f)"
                  >
                    <i class="material-icons">link_off</i>
                  </button>
                </td>
              </tr>
            </table>
          </div>
          <h2 class="message" v-else>
            <i class="material-icons">sync_disabled</i>
            <span>{{ t("sync.noFolders") }}</span>
          </h2>
        </div>

        <div class="card" v-if="conflicts.length > 0">
          <div class="card-title">
            <h2>{{ t("sync.conflicts") }}</h2>
          </div>
          <div class="card-content">
            <p>{{ t("sync.conflictsHelp") }}</p>
          </div>
          <div class="card-content full">
            <table>
              <tr>
                <th>{{ t("settings.path") }}</th>
                <th>{{ t("sync.versions") }}</th>
                <th></th>
              </tr>
              <tr v-for="c in conflicts" :key="c.path">
                <td class="conflict-path">
                  <router-link :to="'/files' + dirOf(c.original)">{{
                    c.original
                  }}</router-link>
                </td>
                <td>
                  <small>
                    {{ t("sync.yourVersion") }}:
                    {{
                      c.originalExists
                        ? filesize(c.originalSize) +
                          ", " +
                          dayjs(c.originalModified).fromNow()
                        : "—"
                    }}<br />
                    {{ t("sync.otherVersion") }}: {{ filesize(c.size) }},
                    {{ dayjs(c.conflictedAt).fromNow() }}
                  </small>
                </td>
                <td class="conflict-actions">
                  <button
                    class="button button--flat"
                    @click="resolveConflict(c, 'original')"
                  >
                    {{ t("sync.keepYours") }}
                  </button>
                  <button
                    class="button button--flat"
                    @click="resolveConflict(c, 'conflict')"
                  >
                    {{ t("sync.keepOther") }}
                  </button>
                </td>
              </tr>
            </table>
          </div>
        </div>

        <div class="card" v-if="limits">
          <div class="card-title">
            <h2>{{ t("sync.bandwidth") }}</h2>
          </div>
          <div class="card-content">
            <p>{{ t("sync.bandwidthHelp") }}</p>
            <label class="field">
              <span>{{ t("sync.downLimit") }}</span>
              <input
                class="input input--block"
                type="number"
                min="0"
                v-model.number="limits.maxRecvKbps"
              />
            </label>
            <label class="field">
              <span>{{ t("sync.upLimit") }}</span>
              <input
                class="input input--block"
                type="number"
                min="0"
                v-model.number="limits.maxSendKbps"
              />
            </label>
            <label class="field">
              <input type="checkbox" v-model="limits.limitBandwidthInLan" />
              {{ t("sync.limitLan") }}
            </label>
            <button class="button" @click="saveLimits">
              {{ t("buttons.update") }}
            </button>
          </div>
        </div>

        <div class="card">
          <div class="card-title">
            <h2>{{ t("sync.devices") }}</h2>
          </div>
          <div class="card-content full" v-if="status.devices.length > 0">
            <table>
              <tr>
                <th>{{ t("sync.deviceName") }}</th>
                <th>{{ t("sync.deviceAddress") }}</th>
                <th>{{ t("sync.state") }}</th>
                <th>{{ t("sync.speed") }}</th>
                <th>{{ t("sync.transferred") }}</th>
                <th></th>
              </tr>
              <tr v-for="d in status.devices" :key="d.id">
                <td>
                  {{ d.name }}<br /><small>{{ d.id.slice(0, 7) }}…</small>
                </td>
                <td>{{ d.address }}</td>
                <td>
                  <span :class="['sync-state', d.connected ? 'ok' : 'off']">{{
                    d.connected ? t("sync.connected") : t("sync.disconnected")
                  }}</span>
                </td>
                <td>
                  <template v-if="d.connected && rates[d.id]">
                    ↓ {{ speed(rates[d.id].down) }}<br />
                    ↑ {{ speed(rates[d.id].up) }}
                  </template>
                  <template v-else>—</template>
                </td>
                <td>
                  <template v-if="d.connected">
                    ↓ {{ filesize(d.inBytes) }}<br />
                    ↑ {{ filesize(d.outBytes) }}
                  </template>
                  <template v-else>—</template>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="t('buttons.delete')"
                    :aria-label="t('buttons.delete')"
                    @click="removeDevice(d)"
                  >
                    <i class="material-icons">delete</i>
                  </button>
                </td>
              </tr>
            </table>
          </div>
          <div class="card-content">
            <h3>{{ t("sync.addDevice") }}</h3>
            <sync-device-form
              :defaultPort="status.defaultPort"
              @added="refresh"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { sync as api } from "@/api";
import { StatusError } from "@/api/utils";
import { copy } from "@/utils/clipboard";
import dayjs from "dayjs";
import { filesize } from "@/utils";
import { transferRate } from "@/utils/syncRate";
import { folderKind, minCompletion } from "@/utils/syncState";
import { useLayoutStore } from "@/stores/layout";
import type { TransferRate, TransferSample } from "@/utils/syncRate";
import Errors from "@/views/Errors.vue";
import SyncDeviceForm from "@/components/settings/SyncDeviceForm.vue";
import PathInput from "@/components/PathInput.vue";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();

const status = ref<SyncStatus | null>(null);

// The Syncthing web interface is published on the same host as this page, on
// its own port; it speaks plain HTTP unless the user turned TLS on there.
const guiUrl = computed(() =>
  status.value?.guiPort
    ? `http://${window.location.hostname}:${status.value.guiPort}/`
    : ""
);
const conflicts = ref<SyncConflict[]>([]);
const limits = ref<SyncLimits | null>(null);
const layoutStore = useLayoutStore();
let polls = 0;
const error = ref<StatusError | null>(null);
const acceptingDevice = ref<string | null>(null);
const acceptingFolder = ref<string | null>(null);
const acceptPath = ref("");
const acceptMode = ref<"receive" | "twoway">("receive");
const rates = ref<Record<string, TransferRate>>({});
const samples = new Map<string, TransferSample>();
let timer: number | undefined;

const refresh = async () => {
  try {
    const next = await api.status();
    updateRates(next);
    status.value = next;
    error.value = null;
  } catch (e) {
    if (status.value === null && e instanceof StatusError) {
      error.value = e;
    }
  }
};

// Remember the byte counters of this poll and derive the speed per device.
const updateRates = (next: SyncStatus) => {
  const updated: Record<string, TransferRate> = {};
  for (const device of next.devices) {
    const sample = {
      time: next.time,
      inBytes: device.inBytes,
      outBytes: device.outBytes,
    };
    const rate = device.connected
      ? transferRate(samples.get(device.id), sample)
      : null;
    if (rate) updated[device.id] = rate;
    samples.set(device.id, sample);
  }
  rates.value = updated;
};

const speed = (bytesPerSecond: number) =>
  `${filesize(Math.round(bytesPerSecond))}/s`;

const totalRate = computed(() => {
  let down = 0;
  let up = 0;
  for (const rate of Object.values(rates.value)) {
    down += rate.down;
    up += rate.up;
  }
  return { down, up };
});

// Looking for conflicts walks the synced folders, so it runs less often.
const refreshConflicts = async () => {
  try {
    conflicts.value = await api.conflicts();
  } catch {
    // Not critical: the rest of the page still works.
  }
};

onMounted(async () => {
  refresh();
  refreshConflicts();
  api
    .limits()
    .then((l) => (limits.value = l))
    .catch(() => {});
  timer = window.setInterval(() => {
    refresh();
    if (++polls % 5 === 0) refreshConflicts();
  }, 4000);
});
onBeforeUnmount(() => window.clearInterval(timer));

// Run an action, then refresh; errors are shown as toasts.
const run = async (action: () => Promise<unknown>, success?: string) => {
  try {
    await action();
    if (success) $showSuccess(success);
  } catch (e) {
    $showError(e as Error);
  }
  await refresh();
};

const copyId = () => {
  copy({ text: status.value?.deviceID ?? "" }).then(
    () => $showSuccess(t("success.linkCopied")),
    () => {}
  );
};

// The source port of an incoming connection is random, so keep only the host.
const hostOf = (address: string) => address.replace(/:\d+$/, "");

const modeLabel = (mode: SyncMode) =>
  ({
    oneway: t("sync.modeOneWay"),
    twoway: t("sync.modeTwoWay"),
    receive: t("sync.modeReceive"),
  })[mode];

const remoteNames = (folder: SyncFolder) =>
  folder.remotes
    .map(
      (r) =>
        status.value?.devices.find((d) => d.id === r.deviceID)?.name ??
        r.deviceID.slice(0, 7)
    )
    .join(", ");

const stateClass = folderKind;

const remaining = (bytes: number, files: number) =>
  bytes > 0
    ? ` · ${t("sync.left", { size: filesize(bytes), count: files }, files)}`
    : "";

const stateLabel = (folder: SyncFolder) => {
  if (folder.error) return `${t("sync.error")}: ${folder.error}`;
  if (folder.paused) return t("sync.paused");
  if (folder.remotes.length > 0 && folder.remotes.every((r) => !r.connected))
    return t("sync.disconnected");
  if (folder.state === "idle" && minCompletion(folder) >= 100)
    return t("sync.upToDate");
  if (folder.state === "idle") {
    const bytes = folder.remotes.reduce((sum, r) => sum + r.needBytes, 0);
    const items = folder.remotes.reduce((sum, r) => sum + r.needItems, 0);
    return (
      t("sync.remoteSyncing", { percent: Math.floor(minCompletion(folder)) }) +
      remaining(bytes, items)
    );
  }
  if (folder.state === "scanning") return t("sync.scanning");
  return t("sync.syncing") + remaining(folder.needBytes, folder.needFiles);
};

const dirOf = (path: string) => path.replace(/\/[^/]*$/, "") + "/";

const resolveConflict = async (
  c: SyncConflict,
  keep: "conflict" | "original"
) => {
  try {
    await api.resolveConflict(c.path, keep);
    $showSuccess(t("sync.conflictResolved"));
  } catch (e) {
    $showError(e as Error);
  }
  await refreshConflicts();
};

const saveLimits = async () => {
  if (!limits.value) return;

  try {
    await api.setLimits({
      maxRecvKbps: limits.value.maxRecvKbps || 0,
      maxSendKbps: limits.value.maxSendKbps || 0,
      limitBandwidthInLan: limits.value.limitBandwidthInLan,
    });
    $showSuccess(t("sync.limitsSaved"));
  } catch (e) {
    $showError(e as Error);
  }
};

const editIgnores = (folder: SyncFolder) =>
  layoutStore.showHover({
    prompt: "syncIgnores",
    props: { id: folder.id, path: folder.path },
  });

const togglePause = (folder: SyncFolder) =>
  run(() => api.setPaused(folder.id, !folder.paused));

const removeFolder = (folder: SyncFolder) => {
  if (!window.confirm(t("sync.confirmStop", { path: folder.path }))) return;
  run(() => api.removeFolder(folder.id));
};

const removeDevice = (device: SyncDevice) => {
  if (!window.confirm(t("sync.confirmRemoveDevice", { name: device.name })))
    return;
  run(() => api.removeDevice(device.id));
};

const dismissDevice = (p: SyncPendingDevice) =>
  run(() => api.dismissPendingDevice(p.id));

const dismissFolder = (p: SyncPendingFolder) =>
  run(() => api.dismissPendingFolder(p.id, p.deviceID));

const startAccept = (p: SyncPendingFolder) => {
  acceptingFolder.value = p.id + p.deviceID;
  acceptPath.value = "/" + p.label.replace(/[\\/]/g, "_");
  acceptMode.value = "receive";
};

const acceptFolder = async (p: SyncPendingFolder) => {
  await run(
    () =>
      api.acceptFolder({
        folderID: p.id,
        deviceID: p.deviceID,
        path: acceptPath.value.trim(),
        mode: acceptMode.value,
      }),
    t("sync.accepted")
  );
  acceptingFolder.value = null;
};
</script>

<style scoped>
.row .column.sync-column {
  width: 100%;
}

.sync-stack {
  flex: 1;
  min-width: 0;
}

.sync-stack .card {
  margin-bottom: 1em;
}

.sync-id {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.sync-id code {
  word-break: break-all;
}

.pending {
  margin-bottom: 1.2em;
  padding-bottom: 1em;
  border-bottom: 1px solid var(--divider);
}

.pending small {
  display: block;
  opacity: 0.6;
  word-break: break-all;
}

.field {
  display: block;
  margin-bottom: 0.8em;
}

.field > span {
  display: block;
  margin-bottom: 0.2em;
  font-size: 0.9em;
  opacity: 0.8;
}

.field small {
  display: block;
  margin-top: 0.2em;
  opacity: 0.7;
}

.conflict-path {
  word-break: break-all;
}

.conflict-actions {
  white-space: nowrap;
}

.sync-summary {
  margin: 0;
}

.sync-speed {
  margin-left: 1em;
  font-weight: bold;
  white-space: nowrap;
}

.sync-state.ok {
  color: #43a047;
}

.sync-state.busy {
  color: #1e88e5;
}

.sync-state.off {
  opacity: 0.6;
}

.sync-state.err {
  color: #e53935;
}
</style>
