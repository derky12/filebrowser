<template>
  <div id="trash">
    <header-bar showMenu showLogo>
      <title>{{ t("trash.title") }}</title>
      <template #actions>
        <action
          v-if="listing && listing.items.length > 0"
          icon="delete_sweep"
          :label="t('trash.emptyTrash')"
          @action="emptyTrash"
        />
      </template>
    </header-bar>

    <div class="trash-content">
      <p v-if="listing" class="trash-usage">
        {{
          t("trash.usage", {
            used: filesize(listing.totalBytes),
            limit: listing.maxBytes > 0 ? filesize(listing.maxBytes) : "∞",
            days: listing.days > 0 ? listing.days : "∞",
          })
        }}
        <template v-if="!listing.enabled"> · {{ t("trash.off") }}</template>
      </p>

      <table v-if="listing && listing.items.length > 0" class="trash-table">
        <tr>
          <th>{{ t("trash.name") }}</th>
          <th>{{ t("trash.original") }}</th>
          <th>{{ t("trash.size") }}</th>
          <th>{{ t("trash.deleted") }}</th>
          <th></th>
          <th></th>
        </tr>
        <tr v-for="item in listing.items" :key="item.id">
          <td>
            <i class="material-icons">{{
              item.isDir ? "folder" : "insert_drive_file"
            }}</i>
            {{ item.name }}
          </td>
          <td class="trash-path">{{ item.path }}</td>
          <td>{{ filesize(item.size) }}</td>
          <td :title="dayjs(item.deletedAt).format('L LT')">
            {{ dayjs(item.deletedAt).fromNow() }}
          </td>
          <td class="small">
            <button
              class="action"
              :title="t('trash.restore')"
              :aria-label="t('trash.restore')"
              @click="restore(item)"
            >
              <i class="material-icons">restore_from_trash</i>
            </button>
          </td>
          <td class="small">
            <button
              class="action"
              :title="t('trash.deleteForever')"
              :aria-label="t('trash.deleteForever')"
              @click="deleteForever(item)"
            >
              <i class="material-icons">delete_forever</i>
            </button>
          </td>
        </tr>
      </table>
      <h2 class="message" v-else-if="listing">
        <i class="material-icons">delete_outline</i>
        <span>{{ t("trash.empty") }}</span>
      </h2>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import { trash as api } from "@/api";
import { StatusError } from "@/api/utils";
import { filesize } from "@/utils";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();

const listing = ref<TrashListing | null>(null);

const refresh = async () => {
  try {
    listing.value = await api.list();
  } catch (e) {
    $showError(e as Error);
  }
};

const restore = async (item: TrashItem) => {
  try {
    const result = await api.restore(item.id);
    $showSuccess(t("trash.restored", { path: result.path }));
  } catch (e) {
    if (e instanceof StatusError && e.status === 409) {
      // Something is already at the original path.
      if (window.confirm(t("trash.restoreConflict", { path: item.path }))) {
        try {
          const result = await api.restore(item.id, true);
          $showSuccess(t("trash.restored", { path: result.path }));
        } catch (err) {
          $showError(err as Error);
        }
      }
    } else {
      $showError(e as Error);
    }
  }
  await refresh();
};

const deleteForever = async (item: TrashItem) => {
  if (!window.confirm(t("trash.confirmForever", { name: item.name }))) return;

  try {
    await api.remove(item.id);
  } catch (e) {
    $showError(e as Error);
  }
  await refresh();
};

const emptyTrash = async () => {
  if (!window.confirm(t("trash.confirmEmpty"))) return;

  try {
    await api.empty();
  } catch (e) {
    $showError(e as Error);
  }
  await refresh();
};

onMounted(refresh);
</script>

<style scoped>
.trash-content {
  padding: 1em;
}

.trash-usage {
  opacity: 0.75;
}

.trash-table {
  width: 100%;
}

.trash-table td,
.trash-table th {
  text-align: left;
  vertical-align: middle;
}

.trash-path {
  word-break: break-all;
  opacity: 0.8;
}

.trash-table i.material-icons {
  vertical-align: middle;
  margin-right: 0.2em;
}
</style>
