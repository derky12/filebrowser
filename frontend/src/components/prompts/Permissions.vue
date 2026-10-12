<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("perms.title") }}</h2>
    </div>

    <div class="card-content">
      <p class="perm-target">
        <template v-if="items.length === 1">{{ items[0].path }}</template>
        <template v-else>{{ t("perms.nItems", { n: items.length }) }}</template>
      </p>
      <p v-if="items.length > 1" class="perm-note">
        {{ t("perms.multiNote") }}
      </p>

      <p v-if="loading">{{ t("files.loading") }}</p>

      <template v-else-if="info">
        <table class="perm-grid">
          <tr>
            <th></th>
            <th>{{ t("perms.read") }}</th>
            <th>{{ t("perms.write") }}</th>
            <th>{{ t("perms.execute") }}</th>
          </tr>
          <tr v-for="(who, row) in WHO" :key="who">
            <th>{{ t("perms." + who) }}</th>
            <td v-for="(bit, col) in BITS" :key="bit">
              <input
                type="checkbox"
                :checked="(mode & bitMask(row, col)) !== 0"
                :aria-label="t('perms.' + who) + ' ' + t('perms.' + bit)"
                @change="toggle(bitMask(row, col))"
              />
            </td>
          </tr>
        </table>

        <label class="perm-field">
          <span>{{ t("perms.octal") }}</span>
          <input
            class="input input--block perm-octal"
            type="text"
            maxlength="4"
            spellcheck="false"
            :value="octal"
            @input="setOctal(($event.target as HTMLInputElement).value)"
          />
        </label>

        <template v-if="info.canChown">
          <label class="perm-field">
            <span>{{ t("perms.owner") }}</span>
            <input
              class="input input--block"
              type="text"
              spellcheck="false"
              v-model.trim="owner"
              :placeholder="String(info.uid)"
              @input="ownerDirty = true"
            />
          </label>
          <label class="perm-field">
            <span>{{ t("perms.group") }}</span>
            <input
              class="input input--block"
              type="text"
              spellcheck="false"
              v-model.trim="group"
              :placeholder="String(info.gid)"
              @input="groupDirty = true"
            />
          </label>
          <p class="perm-note">{{ t("perms.ownerHelp") }}</p>
        </template>

        <template v-if="anyDir">
          <label class="perm-check">
            <input type="checkbox" v-model="recursive" />
            {{ t("perms.recursive") }}
          </label>
          <template v-if="recursive">
            <label class="perm-field">
              <span>{{ t("perms.fileMode") }}</span>
              <input
                class="input input--block perm-octal"
                type="text"
                maxlength="4"
                spellcheck="false"
                v-model.trim="fileMode"
              />
            </label>
            <label class="perm-field">
              <span>{{ t("perms.dirMode") }}</span>
              <input
                class="input input--block perm-octal"
                type="text"
                maxlength="4"
                spellcheck="false"
                v-model.trim="dirMode"
              />
            </label>
            <p class="perm-note">{{ t("perms.recursiveHelp") }}</p>
          </template>
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
        :disabled="!canApply"
        @click="apply"
        :aria-label="t('perms.apply')"
        :title="t('perms.apply')"
      >
        {{ t("perms.apply") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { permissions as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

interface Target {
  path: string;
  name: string;
  isDir: boolean;
}

const WHO = ["owner", "group", "others"] as const;
const BITS = ["read", "write", "execute"] as const;

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const items = computed<Target[]>(
  () => layoutStore.currentPrompt?.props?.items ?? []
);
const anyDir = computed(() => items.value.some((i) => i.isDir));

const loading = ref(true);
const info = ref<PermissionsInfo | null>(null);
const mode = ref(0o644);
const modeDirty = ref(false);
const owner = ref("");
const group = ref("");
const ownerDirty = ref(false);
const groupDirty = ref(false);
const recursive = ref(false);
const fileMode = ref("0644");
const dirMode = ref("0755");

// Row 0..2 = owner, group, others; column 0..2 = read, write, execute.
const bitMask = (row: number, col: number) => 1 << (8 - (row * 3 + col));

const toOctal = (value: number) => value.toString(8).padStart(4, "0");
const octal = computed(() => toOctal(mode.value));

const toggle = (mask: number) => {
  mode.value ^= mask;
  modeDirty.value = true;
};

const setOctal = (text: string) => {
  if (!/^[0-7]{3,4}$/.test(text)) return;

  const value = parseInt(text, 8);
  if (value > 0o777) return;

  mode.value = value;
  modeDirty.value = true;
};

const validOctal = (text: string) =>
  /^[0-7]{3,4}$/.test(text) && parseInt(text, 8) <= 0o777;

const canApply = computed(() => {
  if (loading.value || !info.value) return false;
  if (
    recursive.value &&
    (!validOctal(fileMode.value) || !validOctal(dirMode.value))
  )
    return false;

  return (
    modeDirty.value || ownerDirty.value || groupDirty.value || recursive.value
  );
});

// The prompt is kept alive between uses, so reset and reload each time it is
// shown (onActivated also runs on the first mount).
onActivated(async () => {
  loading.value = true;
  info.value = null;
  modeDirty.value = ownerDirty.value = groupDirty.value = false;
  recursive.value = false;

  try {
    const first = items.value[0];
    info.value = await api.get(first.path);
    mode.value = parseInt(info.value.mode, 8);
    owner.value = info.value.owner || "";
    group.value = info.value.group || "";
    dirMode.value = toOctal(mode.value);
    fileMode.value = toOctal(mode.value & ~0o111);
  } catch (e) {
    $showError(e as Error);
    layoutStore.closeHovers();
  } finally {
    loading.value = false;
  }
});

const apply = async () => {
  let changed = 0;
  let skipped = 0;
  let failed = 0;
  const errors: string[] = [];

  for (const item of items.value) {
    const change: PermissionsChange = {};
    if (modeDirty.value) change.mode = octal.value;
    if (info.value?.canChown && ownerDirty.value) change.owner = owner.value;
    if (info.value?.canChown && groupDirty.value) change.group = group.value;
    if (recursive.value && item.isDir) {
      change.recursive = true;
      change.fileMode = fileMode.value;
      change.dirMode = dirMode.value;
    }

    try {
      const result = await api.set(item.path, change);
      changed += result.changed;
      skipped += result.skipped;
      failed += result.failed;
      errors.push(...result.errors);
    } catch (e) {
      failed++;
      errors.push(`${item.path}: ${(e as Error).message}`);
    }
  }

  fileStore.reload = true;
  layoutStore.closeHovers();

  if (failed > 0) {
    $showError(
      t("perms.partial", { changed, failed }) +
        (errors.length > 0 ? ` ${errors.slice(0, 3).join("; ")}` : "")
    );
  } else {
    $showSuccess(
      skipped > 0
        ? t("perms.doneSkipped", { n: changed, skipped })
        : t("perms.done", { n: changed })
    );
  }
};
</script>

<style scoped>
.perm-target {
  margin-top: 0;
  word-break: break-all;
  opacity: 0.8;
}

.perm-note {
  margin: 0.4em 0;
  font-size: 0.85em;
  opacity: 0.7;
}

.perm-grid {
  width: 100%;
  margin-bottom: 1em;
  text-align: center;
}

.perm-grid th:first-child {
  text-align: left;
}

.perm-field {
  display: block;
  margin-bottom: 0.8em;
}

.perm-field > span {
  display: block;
  margin-bottom: 0.2em;
  font-size: 0.9em;
  opacity: 0.8;
}

.perm-octal {
  font-family: monospace;
}

.perm-check {
  display: block;
  margin: 1em 0 0.6em;
}
</style>
