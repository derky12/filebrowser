<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("archive.extract") }}</h2>
    </div>

    <div class="card-content">
      <p class="arch-note">{{ archive?.path }}</p>
      <label class="arch-field">
        <span>{{ t("archive.intoFolder") }}</span>
        <input
          id="focus-prompt"
          class="input input--block"
          type="text"
          v-model.trim="destination"
          spellcheck="false"
          @keyup.enter="submit"
        />
        <small>{{ t("archive.intoHelp") }}</small>
      </label>
      <label class="arch-check">
        <input type="checkbox" v-model="overwrite" />
        {{ t("archive.overwrite") }}
      </label>
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
        :disabled="busy || destination === ''"
        @click="submit"
        :aria-label="t('archive.extract')"
        :title="t('archive.extract')"
      >
        {{ busy ? t("archive.working") : t("archive.extract") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { archive as api } from "@/api";
import { archiveBaseName } from "@/utils/archives";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const archive = computed<{ path: string; name: string; folder: string } | null>(
  () => layoutStore.currentPrompt?.props?.archive ?? null
);

const destination = ref("");
const overwrite = ref(false);
const busy = ref(false);

// By default into a new folder named after the archive, next to it.
onActivated(() => {
  busy.value = false;
  overwrite.value = false;
  if (archive.value) {
    const folder = archive.value.folder === "/" ? "" : archive.value.folder;
    destination.value = `${folder}/${archiveBaseName(archive.value.name)}`;
  }
});

const submit = async () => {
  if (busy.value || !archive.value || destination.value === "") return;

  busy.value = true;
  try {
    const result = await api.extract(
      archive.value.path,
      destination.value.startsWith("/")
        ? destination.value
        : `/${destination.value}`,
      overwrite.value
    );
    $showSuccess(
      result.skipped > 0
        ? t("archive.extractedSkipped", {
            n: result.files,
            skipped: result.skipped,
          })
        : t("archive.extracted", { n: result.files, path: result.path })
    );
    fileStore.reload = true;
    layoutStore.closeHovers();
  } catch (e) {
    $showError(e as Error);
    busy.value = false;
  }
};
</script>

<style scoped>
.arch-note {
  margin-top: 0;
  opacity: 0.75;
  word-break: break-all;
}

.arch-field {
  display: block;
  margin-bottom: 0.8em;
}

.arch-field > span {
  display: block;
  margin-bottom: 0.2em;
  font-size: 0.9em;
  opacity: 0.8;
}

.arch-field small {
  display: block;
  margin-top: 0.2em;
  opacity: 0.7;
}

.arch-check {
  display: block;
}
</style>
