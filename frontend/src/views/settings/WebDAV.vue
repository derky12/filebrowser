<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="info">
    <div class="column dav-column">
      <div class="dav-stack">
        <div class="card">
          <div class="card-title">
            <h2>{{ t("webdav.title") }}</h2>
          </div>
          <div class="card-content">
            <p>{{ t("webdav.intro") }}</p>
            <p>
              <strong>{{ t("webdav.address") }}:</strong>
              <code>{{ info.baseURL }}/</code>
              <button
                class="action"
                :title="t('buttons.copyToClipboard')"
                :aria-label="t('buttons.copyToClipboard')"
                @click="copyText(info.baseURL + '/')"
              >
                <i class="material-icons">content_paste</i>
              </button>
            </p>
            <p>
              <strong>{{ t("webdav.userName") }}:</strong>
              <code>{{ info.user }}</code>
              <br />
              <small>{{ t("webdav.passwordNote") }}</small>
            </p>
          </div>
        </div>

        <div class="card">
          <div class="card-title">
            <h2>{{ t("webdav.shares") }}</h2>
          </div>
          <div class="card-content full" v-if="info.shares.length > 0">
            <table>
              <tr>
                <th>{{ t("webdav.name") }}</th>
                <th>{{ t("settings.path") }}</th>
                <th>{{ t("webdav.mode") }}</th>
                <th></th>
                <th></th>
              </tr>
              <tr v-for="s in info.shares" :key="s.name">
                <td>
                  {{ s.name }}
                  <small v-if="!s.available" class="dav-missing">{{
                    t("webdav.missing")
                  }}</small>
                </td>
                <td class="dav-path">
                  <router-link v-if="s.available" :to="'/files' + s.path">{{
                    s.path
                  }}</router-link>
                  <template v-else>{{ s.path }}</template>
                </td>
                <td>
                  <select
                    class="input"
                    :value="s.readOnly ? 'ro' : 'rw'"
                    @change="
                      changeMode(
                        s,
                        ($event.target as HTMLSelectElement).value === 'ro'
                      )
                    "
                  >
                    <option value="ro">{{ t("webdav.readOnlyShort") }}</option>
                    <option value="rw">{{ t("webdav.readWriteShort") }}</option>
                  </select>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="t('webdav.copyUrl')"
                    :aria-label="t('webdav.copyUrl')"
                    @click="copyText(s.url)"
                  >
                    <i class="material-icons">link</i>
                  </button>
                </td>
                <td class="small">
                  <button
                    class="action"
                    :title="t('webdav.stop')"
                    :aria-label="t('webdav.stop')"
                    @click="stop(s)"
                  >
                    <i class="material-icons">link_off</i>
                  </button>
                </td>
              </tr>
            </table>
          </div>
          <h2 class="message" v-else>
            <i class="material-icons">cloud_off</i>
            <span>{{ t("webdav.none") }}</span>
          </h2>
        </div>

        <div class="card">
          <div class="card-title">
            <h2>{{ t("webdav.howTo") }}</h2>
          </div>
          <div class="card-content">
            <webdav-help :url="info.baseURL + '/'" :user="info.user" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { webdav as api } from "@/api";
import { StatusError } from "@/api/utils";
import { copy } from "@/utils/clipboard";
import Errors from "@/views/Errors.vue";
import WebdavHelp from "@/components/settings/WebdavHelp.vue";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();

const info = ref<WebdavInfo | null>(null);
const error = ref<StatusError | null>(null);

const load = async () => {
  try {
    info.value = await api.info();
  } catch (e) {
    if (e instanceof StatusError) error.value = e;
    else $showError(e as Error);
  }
};

const copyText = (text: string) =>
  copy({ text }).then(
    () => $showSuccess(t("success.linkCopied")),
    () => {}
  );

const changeMode = async (share: WebdavShare, readOnly: boolean) => {
  try {
    await api.setReadOnly(share.name, readOnly);
    $showSuccess(t("webdav.modeSaved"));
  } catch (e) {
    $showError(e as Error);
  }
  await load();
};

const stop = async (share: WebdavShare) => {
  if (!window.confirm(t("webdav.confirmStop", { name: share.name }))) return;

  try {
    await api.removeShare(share.name);
  } catch (e) {
    $showError(e as Error);
  }
  await load();
};

onMounted(load);
</script>

<style scoped>
.row .column.dav-column {
  width: 100%;
}

.dav-stack {
  flex: 1;
  min-width: 0;
}

.dav-stack .card {
  margin-bottom: 1em;
}

.dav-path {
  word-break: break-all;
}

.dav-missing {
  display: block;
  color: #e53935;
}

code {
  word-break: break-all;
}
</style>
