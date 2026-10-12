<template>
  <div class="webdav-help">
    <p>
      {{ t("webdav.helpIntro", { user }) }}
    </p>
    <details v-for="item in items" :key="item.key">
      <summary>{{ t("webdav.help." + item.key + ".title") }}</summary>
      <p>{{ t("webdav.help." + item.key + ".text", { url }) }}</p>
      <pre v-if="item.cmd">{{
        t("webdav.help." + item.key + ".cmd", { url, user })
      }}</pre>
    </details>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

defineProps<{ url: string; user: string }>();

const { t } = useI18n();

const items = [
  { key: "mac", cmd: false },
  { key: "windows", cmd: true },
  { key: "linux", cmd: true },
  { key: "phone", cmd: false },
];
</script>

<style scoped>
.webdav-help details {
  margin: 0.4em 0;
}

.webdav-help summary {
  cursor: pointer;
  font-weight: bold;
}

.webdav-help pre {
  margin: 0.4em 0;
  padding: 0.5em 0.7em;
  overflow-x: auto;
  background: var(--divider);
  border-radius: 4px;
  font-size: 0.9em;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
