<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ t("webdav.shareTitle") }}</h2>
    </div>

    <div class="card-content" v-if="!created">
      <p class="dav-path">{{ path }}</p>
      <label class="dav-field">
        <span>{{ t("webdav.name") }}</span>
        <input
          id="focus-prompt"
          class="input input--block"
          type="text"
          v-model="name"
          spellcheck="false"
          @keyup.enter="submit"
        />
        <small>{{ t("webdav.nameHelp") }}</small>
      </label>
      <label class="dav-radio">
        <input type="radio" :value="true" v-model="readOnly" />
        {{ t("webdav.readOnly") }}
      </label>
      <label class="dav-radio">
        <input type="radio" :value="false" v-model="readOnly" />
        {{ t("webdav.readWrite") }}
      </label>
      <p v-if="!readOnly" class="dav-warning">{{ t("webdav.writeWarning") }}</p>
    </div>

    <div class="card-content" v-else>
      <p>{{ t("webdav.created") }}</p>
      <div class="dav-url">
        <input
          class="input input--block"
          type="text"
          readonly
          :value="created.url"
        />
        <button
          class="action"
          :title="t('buttons.copyToClipboard')"
          :aria-label="t('buttons.copyToClipboard')"
          @click="copyUrl"
        >
          <i class="material-icons">content_paste</i>
        </button>
      </div>
      <p class="dav-note">{{ t("webdav.login", { user }) }}</p>
      <webdav-help :url="created.url" :user="user" />
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="layoutStore.closeHovers"
        :aria-label="created ? t('buttons.close') : t('buttons.cancel')"
        :title="created ? t('buttons.close') : t('buttons.cancel')"
      >
        {{ created ? t("buttons.close") : t("buttons.cancel") }}
      </button>
      <button
        v-if="!created"
        class="button button--flat"
        :disabled="busy || name.trim() === ''"
        @click="submit"
        :aria-label="t('webdav.share')"
        :title="t('webdav.share')"
      >
        {{ t("webdav.share") }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, ref } from "vue";
import { useI18n } from "vue-i18n";
import { webdav as api } from "@/api";
import { copy } from "@/utils/clipboard";
import { useLayoutStore } from "@/stores/layout";
import WebdavHelp from "@/components/settings/WebdavHelp.vue";

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const layoutStore = useLayoutStore();

const path = computed<string>(() => layoutStore.currentPrompt?.props?.path);

const name = ref("");
const readOnly = ref(true);
const busy = ref(false);
const created = ref<WebdavShare | null>(null);
const user = ref("webdav");

// Kept alive between uses: start over every time it is shown.
onActivated(async () => {
  created.value = null;
  busy.value = false;
  readOnly.value = true;
  name.value = (path.value ?? "").split("/").filter(Boolean).pop() ?? "";
  try {
    user.value = (await api.info()).user;
  } catch {
    // The user name is only shown as a hint.
  }
});

const submit = async () => {
  if (busy.value || name.value.trim() === "") return;

  busy.value = true;
  try {
    created.value = await api.createShare({
      path: path.value,
      name: name.value.trim(),
      readOnly: readOnly.value,
    });
  } catch (e) {
    $showError(e as Error);
  } finally {
    busy.value = false;
  }
};

const copyUrl = () => {
  if (!created.value) return;
  copy({ text: created.value.url }).then(
    () => $showSuccess(t("success.linkCopied")),
    () => {}
  );
};
</script>

<style scoped>
.dav-path {
  margin-top: 0;
  opacity: 0.7;
  word-break: break-all;
}

.dav-field {
  display: block;
  margin-bottom: 0.8em;
}

.dav-field > span {
  display: block;
  margin-bottom: 0.2em;
  font-size: 0.9em;
  opacity: 0.8;
}

.dav-field small {
  display: block;
  margin-top: 0.2em;
  opacity: 0.7;
}

.dav-radio {
  display: block;
  margin: 0.3em 0;
}

.dav-warning {
  color: #fb8c00;
  font-size: 0.9em;
}

.dav-url {
  display: flex;
  align-items: center;
  gap: 0.4em;
}

.dav-note {
  font-size: 0.9em;
  opacity: 0.8;
}
</style>
