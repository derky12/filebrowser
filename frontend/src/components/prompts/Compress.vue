<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("archive.compress") }}</h2>
    </div>

    <div class="card-content">
      <p class="arch-note">
        {{ t("archive.compressing", { n: items.length }) }}
      </p>
      <label class="arch-field">
        <span>{{ t("archive.name") }}</span>
        <input
          id="focus-prompt"
          class="input input--block"
          type="text"
          v-model.trim="name"
          @keyup.enter="submit"
        />
      </label>
      <label class="arch-field">
        <span>{{ t("archive.format") }}</span>
        <select class="input input--block" v-model="format">
          <option v-for="f in FORMATS" :key="f.value" :value="f.value">
            {{ f.label }}
          </option>
        </select>
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
        :disabled="busy || name === ''"
        @click="submit"
        :aria-label="t('archive.compress')"
        :title="t('archive.compress')"
      >
        {{ busy ? t("archive.working") : t("archive.compress") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { archive as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

const FORMATS = [
  { value: "zip", label: "ZIP (.zip)" },
  { value: "tar.gz", label: "tar + gzip (.tar.gz)" },
  { value: "tar.xz", label: "tar + xz (.tar.xz)" },
  { value: "tar.zst", label: "tar + zstd (.tar.zst)" },
  { value: "tar.bz2", label: "tar + bzip2 (.tar.bz2)" },
  { value: "tar", label: "tar (.tar)" },
];

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const items = computed<{ path: string; name: string }[]>(
  () => layoutStore.currentPrompt?.props?.items ?? []
);
const folder = computed<string>(
  () => layoutStore.currentPrompt?.props?.folder ?? "/"
);

const name = ref("");
const format = ref("zip");
const busy = ref(false);

onActivated(() => {
  busy.value = false;
  format.value = "zip";
  const first = items.value[0];
  name.value =
    items.value.length === 1 && first
      ? first.name
      : folder.value.split("/").filter(Boolean).pop() || "archive";
});

const submit = async () => {
  if (busy.value || name.value === "") return;

  busy.value = true;
  try {
    const result = await api.compress(
      items.value.map((i) => i.path),
      `${folder.value === "/" ? "" : folder.value}/${name.value}`,
      format.value
    );
    $showSuccess(
      t("archive.compressed", { path: result.path, n: result.files })
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
</style>
