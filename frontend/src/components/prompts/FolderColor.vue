<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("prompts.folderColor") }}</h2>
    </div>

    <div class="card-content">
      <p class="folder-color-path">{{ path }}</p>
      <div class="folder-color-swatches">
        <button
          v-for="color in FOLDER_COLORS"
          :key="color.hex"
          class="folder-color-swatch"
          :class="{ selected: current === color.hex }"
          :style="{ background: color.hex }"
          :title="color.name"
          :aria-label="color.name"
          @click="choose(color.hex)"
        ></button>
      </div>
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
        :disabled="current === undefined"
        :aria-label="t('prompts.folderColorDefault')"
        :title="t('prompts.folderColorDefault')"
        @click="choose(null)"
      >
        {{ t("prompts.folderColorDefault") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { FOLDER_COLORS } from "@/utils/folderColors";

const $showError = inject<IToastError>("$showError")!;

const { t } = useI18n();
const authStore = useAuthStore();
const layoutStore = useLayoutStore();

const path = computed<string>(() => layoutStore.currentPrompt?.props?.path);
const current = computed(() => authStore.folderColor(path.value));

const choose = (color: string | null) => {
  authStore.setFolderColor(path.value, color).catch($showError);
  layoutStore.closeHovers();
};
</script>

<style scoped>
.folder-color-path {
  margin-top: 0;
  word-break: break-all;
  opacity: 0.7;
}

.folder-color-swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6em;
}

.folder-color-swatch {
  width: 2.4em;
  height: 2.4em;
  border: 2px solid transparent;
  border-radius: 50%;
  cursor: pointer;
}

.folder-color-swatch.selected {
  border-color: var(--textPrimary);
  box-shadow: 0 0 0 2px var(--surfacePrimary) inset;
}
</style>
