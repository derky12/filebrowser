<template>
  <div id="disk-usage">
    <header-bar showMenu showLogo>
      <title>{{ t("du.title") }}</title>
      <template #actions>
        <action
          icon="folder_open"
          :label="t('du.openInFiles')"
          @action="openInFiles"
        />
        <action icon="refresh" :label="t('du.refresh')" @action="load" />
      </template>
    </header-bar>

    <div class="du-content">
      <div class="du-crumbs" role="navigation" aria-label="path">
        <router-link to="/du/">/</router-link>
        <template v-for="crumb in crumbs" :key="crumb.path">
          <span>›</span>
          <router-link :to="duLink(crumb.path)">{{ crumb.name }}</router-link>
        </template>
      </div>

      <p v-if="loading" class="du-status">
        <span class="spinner small"></span> {{ t("du.measuring") }}
      </p>

      <template v-if="result">
        <p class="du-summary">
          <strong>{{ filesize(result.total) }}</strong>
          · {{ t("du.files", { n: result.files }) }}
        </p>
        <p v-if="result.partial" class="du-warning">{{ t("du.partial") }}</p>

        <p v-if="result.entries.length === 0" class="du-status">
          {{ t("du.empty") }}
        </p>

        <ul class="du-list">
          <li v-for="e in result.entries" :key="e.path">
            <component
              :is="e.isDir ? 'router-link' : 'a'"
              class="du-row"
              :class="{ file: !e.isDir }"
              :to="e.isDir ? duLink(e.path) : undefined"
              @click="e.isDir ? undefined : openFile(e)"
            >
              <span class="du-bar" :style="{ width: percent(e) + '%' }"></span>
              <span class="du-name">
                <i class="material-icons">{{
                  e.isDir ? "folder" : "insert_drive_file"
                }}</i>
                {{ e.name }}
              </span>
              <span class="du-files" v-if="e.isDir">{{
                t("du.files", { n: e.files })
              }}</span>
              <span class="du-size"
                >{{ e.complete ? "" : "≥ " }}{{ filesize(e.size) }}</span
              >
              <span class="du-percent">{{ percent(e).toFixed(1) }}%</span>
            </component>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { du as api } from "@/api";
import { filesize } from "@/utils";
import { encodePath } from "@/utils/url";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";

const $showError = inject<IToastError>("$showError")!;
const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const result = ref<DuResponse | null>(null);
const loading = ref(false);
let controller: AbortController | null = null;

// The folder being looked at, from the url: /du/a/b/ is /a/b.
const path = computed(() => {
  const raw = (route.params.path as string[] | string | undefined) ?? [];
  const parts = Array.isArray(raw) ? raw : raw.split("/");
  return "/" + parts.filter(Boolean).map(decodeURIComponent).join("/");
});

const crumbs = computed(() => {
  const parts = path.value.split("/").filter(Boolean);
  return parts.map((name, i) => ({
    name,
    path: "/" + parts.slice(0, i + 1).join("/"),
  }));
});

const duLink = (p: string) => `/du${encodePath(p)}/`;

const percent = (e: DuEntry) =>
  result.value && result.value.total > 0
    ? (e.size / result.value.total) * 100
    : 0;

const load = async () => {
  controller?.abort();
  controller = new AbortController();
  loading.value = true;
  result.value = null;

  try {
    result.value = await api.usage(path.value, controller.signal);
  } catch (e) {
    if (!(e as { is_canceled?: boolean }).is_canceled) $showError(e as Error);
  } finally {
    loading.value = false;
  }
};

const openInFiles = () => router.push(`/files${encodePath(path.value)}/`);

const openFile = (e: DuEntry) => router.push(`/files${encodePath(e.path)}`);

watch(path, load, { immediate: true });
onBeforeUnmount(() => controller?.abort());
</script>

<style scoped>
.du-content {
  padding: 1em;
}

.du-crumbs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4em;
  margin-bottom: 0.8em;
}

.du-summary {
  font-size: 1.1em;
  margin: 0.4em 0;
}

.du-warning {
  color: #fb8c00;
}

.du-status {
  opacity: 0.7;
}

.du-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.du-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 1em;
  padding: 0.55em 0.6em;
  margin-bottom: 2px;
  overflow: hidden;
  border-radius: 4px;
  color: var(--textPrimary);
  text-decoration: none;
  cursor: pointer;
}

.du-row.file {
  cursor: default;
}

.du-row:hover {
  background: var(--hover);
}

.du-bar {
  position: absolute;
  inset: 0 auto 0 0;
  background: var(--blue);
  opacity: 0.18;
  pointer-events: none;
}

.du-name {
  position: relative;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.du-name i {
  vertical-align: middle;
  margin-right: 0.3em;
}

.du-files,
.du-percent {
  position: relative;
  opacity: 0.65;
  font-size: 0.9em;
  white-space: nowrap;
}

.du-size {
  position: relative;
  min-width: 6em;
  text-align: right;
  font-weight: bold;
  white-space: nowrap;
}

@media (max-width: 736px) {
  .du-files {
    display: none;
  }
}
</style>
