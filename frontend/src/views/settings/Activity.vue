<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else>
    <div class="column activity-column">
      <div class="card">
        <div class="card-title">
          <h2>{{ t("activity.title") }}</h2>
        </div>
        <div class="card-content">
          <p>{{ t("activity.help") }}</p>
          <div class="activity-filters">
            <select class="input" v-model="action" @change="reload">
              <option value="">{{ t("activity.allActions") }}</option>
              <option v-for="a in ACTIONS" :key="a" :value="a">
                {{ actionLabel(a) }}
              </option>
            </select>
            <input
              class="input"
              type="text"
              v-model.trim="user"
              :placeholder="t('activity.user')"
              @keyup.enter="reload"
            />
            <input
              class="input"
              type="text"
              v-model.trim="query"
              :placeholder="t('activity.search')"
              @keyup.enter="reload"
            />
            <button class="button" @click="reload">
              {{ t("activity.apply") }}
            </button>
            <label class="activity-auto">
              <input type="checkbox" v-model="auto" />
              {{ t("activity.autoRefresh") }}
            </label>
          </div>
        </div>

        <div class="card-content full" v-if="items.length > 0">
          <table class="activity-table">
            <tr>
              <th>{{ t("activity.time") }}</th>
              <th>{{ t("activity.user") }}</th>
              <th>{{ t("activity.action") }}</th>
              <th>{{ t("settings.path") }}</th>
              <th>{{ t("activity.address") }}</th>
            </tr>
            <tr v-for="e in items" :key="e.id">
              <td :title="dayjs(e.time).fromNow()">
                {{ dayjs(e.time).format("L LT") }}
              </td>
              <td>{{ e.username || "—" }}</td>
              <td>
                <span class="activity-action" :class="kind(e.action)">{{
                  actionLabel(e.action)
                }}</span>
              </td>
              <td class="activity-path">
                <router-link
                  v-if="e.path && isFilePath(e)"
                  :to="'/files' + e.path"
                  >{{ e.path }}</router-link
                >
                <template v-else>{{ e.path }}</template>
                <template v-if="e.dest"> → {{ e.dest }}</template>
                <small v-if="e.detail">{{ e.detail }}</small>
              </td>
              <td>{{ e.ip }}</td>
            </tr>
          </table>
        </div>
        <h2 class="message" v-else-if="!loading">
          <i class="material-icons">history</i>
          <span>{{ t("activity.empty") }}</span>
        </h2>

        <div class="card-content" v-if="items.length < total">
          <button class="button" @click="more" :disabled="loading">
            {{ t("activity.more", { shown: items.length, total }) }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import { activity as api } from "@/api";
import { StatusError } from "@/api/utils";
import Errors from "@/views/Errors.vue";

const $showError = inject<IToastError>("$showError")!;
const { t, te } = useI18n();

const PAGE = 50;
const ACTIONS = [
  "mkdir",
  "upload",
  "save",
  "delete",
  "rename",
  "move",
  "copy",
  "share",
  "login",
  "login_failed",
  "permissions",
  "trash",
  "restore",
  "purge",
  "compress",
  "extract",
  "sync_share",
  "sync_accept",
  "sync_stop",
  "sync_pause",
  "sync_resume",
  "sync_device_add",
  "sync_device_remove",
];

const items = ref<ActivityEntry[]>([]);
const total = ref(0);
const loading = ref(false);
const error = ref<StatusError | null>(null);
const action = ref("");
const user = ref("");
const query = ref("");
const auto = ref(false);
let timer: number | undefined;

const actionLabel = (name: string) =>
  te(`activity.actions.${name}`) ? t(`activity.actions.${name}`) : name;

// Colors group the actions by how much they matter.
const kind = (name: string) => {
  if (["delete", "purge", "login_failed", "sync_stop"].includes(name))
    return "danger";
  if (["permissions", "move", "rename", "save", "restore"].includes(name))
    return "change";
  return "";
};

// Only actions on files link to the file browser.
const isFilePath = (e: ActivityEntry) =>
  !e.action.startsWith("login") && e.path.startsWith("/");

const fetchPage = async (offset: number) => {
  loading.value = true;
  try {
    return await api.list({
      user: user.value,
      action: action.value,
      q: query.value,
      limit: PAGE,
      offset,
    });
  } catch (e) {
    if (e instanceof StatusError && items.value.length === 0) error.value = e;
    else $showError(e as Error);
    return null;
  } finally {
    loading.value = false;
  }
};

const reload = async () => {
  const page = await fetchPage(0);
  if (page) {
    items.value = page.items;
    total.value = page.total;
  }
};

const more = async () => {
  const page = await fetchPage(items.value.length);
  if (page) {
    items.value = [...items.value, ...page.items];
    total.value = page.total;
  }
};

watch(auto, (on) => {
  window.clearInterval(timer);
  if (on) timer = window.setInterval(reload, 5000);
});

onMounted(reload);
onBeforeUnmount(() => window.clearInterval(timer));
</script>

<style scoped>
.row .column.activity-column {
  width: 100%;
}

.activity-filters {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6em;
  align-items: center;
}

.activity-filters .input {
  width: auto;
  min-width: 10em;
}

.activity-auto {
  margin-left: 0.5em;
  font-size: 0.9em;
}

.activity-table td,
.activity-table th {
  vertical-align: top;
}

.activity-path {
  word-break: break-all;
}

.activity-path small {
  display: block;
  opacity: 0.65;
}

.activity-action {
  white-space: nowrap;
}

.activity-action.danger {
  color: #e53935;
}

.activity-action.change {
  color: #fb8c00;
}
</style>
