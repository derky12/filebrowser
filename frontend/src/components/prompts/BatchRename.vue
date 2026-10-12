<template>
  <div class="card floating batch-card">
    <div class="card-title">
      <h2>{{ t("batch.title", { n: items.length }) }}</h2>
    </div>

    <div class="card-content">
      <div class="batch-grid">
        <label>
          <span>{{ t("batch.find") }}</span>
          <input
            class="input input--block"
            type="text"
            v-model="o.find"
            spellcheck="false"
          />
        </label>
        <label>
          <span>{{ t("batch.replace") }}</span>
          <input
            class="input input--block"
            type="text"
            v-model="o.replace"
            spellcheck="false"
          />
        </label>
        <label class="batch-check">
          <input type="checkbox" v-model="o.useRegex" /> {{ t("batch.regex") }}
        </label>
        <label class="batch-check">
          <input type="checkbox" v-model="o.matchCase" />
          {{ t("batch.matchCase") }}
        </label>
        <label>
          <span>{{ t("batch.prefix") }}</span>
          <input class="input input--block" type="text" v-model="o.prefix" />
        </label>
        <label>
          <span>{{ t("batch.suffix") }}</span>
          <input class="input input--block" type="text" v-model="o.suffix" />
        </label>
        <label>
          <span>{{ t("batch.case") }}</span>
          <select class="input input--block" v-model="o.caseMode">
            <option value="none">{{ t("batch.caseKeep") }}</option>
            <option value="lower">{{ t("batch.caseLower") }}</option>
            <option value="upper">{{ t("batch.caseUpper") }}</option>
            <option value="title">{{ t("batch.caseTitle") }}</option>
          </select>
        </label>
        <label class="batch-check">
          <input type="checkbox" v-model="o.numbering" />
          {{ t("batch.numbering") }}
        </label>
        <template v-if="o.numbering">
          <label>
            <span>{{ t("batch.start") }}</span>
            <input
              class="input input--block"
              type="number"
              v-model.number="o.start"
            />
          </label>
          <label>
            <span>{{ t("batch.step") }}</span>
            <input
              class="input input--block"
              type="number"
              v-model.number="o.step"
            />
          </label>
          <label>
            <span>{{ t("batch.digits") }}</span>
            <input
              class="input input--block"
              type="number"
              min="1"
              max="9"
              v-model.number="o.digits"
            />
          </label>
          <label>
            <span>{{ t("batch.separator") }}</span>
            <input
              class="input input--block"
              type="text"
              v-model="o.separator"
            />
          </label>
          <label class="batch-check">
            <input type="checkbox" v-model="o.numberAtStart" />
            {{ t("batch.numberAtStart") }}
          </label>
        </template>
      </div>

      <p class="batch-summary" :class="{ bad: failed }">
        {{
          failed
            ? t("batch.problems")
            : t("batch.willRename", { n: changed.length, total: rows.length })
        }}
      </p>

      <div class="batch-preview">
        <table>
          <tr
            v-for="row in rows.slice(0, 300)"
            :key="row.oldName"
            :class="{ unchanged: !row.changed, bad: row.error }"
          >
            <td>{{ row.oldName }}</td>
            <td>→</td>
            <td>
              {{ row.newName }}
              <small v-if="row.error">{{
                t("batch.error." + row.error)
              }}</small>
            </td>
          </tr>
        </table>
        <p v-if="rows.length > 300" class="batch-more">
          {{ t("batch.more", { n: rows.length - 300 }) }}
        </p>
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
        :disabled="busy || failed || changed.length === 0"
        @click="apply"
        :aria-label="t('batch.apply')"
        :title="t('batch.apply')"
      >
        {{ busy ? t("archive.working") : t("batch.apply") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { files as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import {
  changedRows,
  defaultOptions,
  hasErrors,
  planRenames,
  renameSteps,
} from "@/utils/batchRename";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const items = computed<{ name: string; isDir: boolean }[]>(
  () => layoutStore.currentPrompt?.props?.items ?? []
);
// The (encoded) url of the folder, with a trailing slash.
const folderUrl = computed<string>(
  () => layoutStore.currentPrompt?.props?.folderUrl ?? "/files/"
);

const o = reactive(defaultOptions());
const busy = ref(false);

onActivated(() => {
  Object.assign(o, defaultOptions());
  busy.value = false;
});

const existing = computed(
  () => new Set((fileStore.req?.items ?? []).map((i) => i.name))
);
const rows = computed(() => planRenames(items.value, o, existing.value));
const changed = computed(() => changedRows(rows.value));
const failed = computed(() => hasErrors(rows.value));

const apply = async () => {
  busy.value = true;

  // Everything is read up front: renaming closes this prompt, after which its
  // props are gone.
  const base = folderUrl.value;
  const steps = renameSteps(rows.value);
  const count = changed.value.length;
  const url = (name: string) => base + encodeURIComponent(name);

  try {
    for (const step of steps) {
      await api.move(
        [{ from: url(step.from), to: url(step.to) }],
        false,
        false
      );
    }
    $showSuccess(t("batch.done", { n: count }));
  } catch (e) {
    $showError(e as Error);
  }

  fileStore.reload = true;
  layoutStore.closeHovers();
  busy.value = false;
};
</script>

<style scoped>
.batch-card {
  max-width: 44em;
}

.batch-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6em 1em;
}

.batch-grid label > span {
  display: block;
  font-size: 0.85em;
  opacity: 0.8;
  margin-bottom: 0.15em;
}

.batch-check {
  align-self: end;
}

.batch-summary {
  margin: 1em 0 0.4em;
  font-weight: bold;
}

.batch-summary.bad {
  color: #e53935;
}

.batch-preview {
  max-height: 14em;
  overflow: auto;
  border: 1px solid var(--divider);
  border-radius: 4px;
  padding: 0.3em 0.6em;
}

.batch-preview table {
  width: 100%;
  font-size: 0.9em;
}

.batch-preview td {
  padding: 0.15em 0.3em;
  word-break: break-all;
}

.batch-preview tr.unchanged {
  opacity: 0.5;
}

.batch-preview tr.bad td {
  color: #e53935;
}

.batch-preview small {
  display: block;
}

.batch-more {
  opacity: 0.7;
  font-size: 0.85em;
}

@media (max-width: 736px) {
  .batch-grid {
    grid-template-columns: 1fr;
  }
}
</style>
