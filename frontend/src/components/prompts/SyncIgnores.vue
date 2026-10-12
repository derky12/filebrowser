<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("sync.ignoreRules") }}</h2>
    </div>

    <div class="card-content">
      <p class="ignore-path">{{ path }}</p>
      <p class="ignore-help">{{ t("sync.ignoreHelp") }}</p>
      <textarea
        class="input input--block ignore-text"
        v-model="text"
        :disabled="loading"
        spellcheck="false"
        rows="10"
        :placeholder="t('sync.ignoreExample')"
      ></textarea>
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
        :disabled="loading"
        @click="save"
        :aria-label="t('buttons.save')"
        :title="t('buttons.save')"
      >
        {{ t("buttons.save") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { sync as api } from "@/api";
import { useLayoutStore } from "@/stores/layout";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const layoutStore = useLayoutStore();

const id = computed<string>(() => layoutStore.currentPrompt?.props?.id);
const path = computed<string>(() => layoutStore.currentPrompt?.props?.path);

const text = ref("");
const loading = ref(true);

// Kept alive between uses: reload every time it is shown.
onActivated(async () => {
  loading.value = true;
  text.value = "";
  try {
    text.value = (await api.ignores(id.value)).lines.join("\n");
  } catch (e) {
    $showError(e as Error);
    layoutStore.closeHovers();
  } finally {
    loading.value = false;
  }
});

const save = async () => {
  const lines = text.value.replace(/\s+$/, "").split("\n");
  try {
    await api.setIgnores(id.value, text.value.trim() === "" ? [] : lines);
    $showSuccess(t("sync.ignoresSaved"));
    layoutStore.closeHovers();
  } catch (e) {
    $showError(e as Error);
  }
};
</script>

<style scoped>
.ignore-path {
  margin-top: 0;
  opacity: 0.7;
  word-break: break-all;
}

.ignore-help {
  font-size: 0.9em;
  opacity: 0.8;
}

.ignore-text {
  font-family: monospace;
  white-space: pre;
}
</style>
