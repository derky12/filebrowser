<template>
  <div class="card floating">
    <div class="card-content">
      <p v-if="!this.isListing || selectedCount === 1">
        {{ $t("prompts.deleteMessageSingle") }}
      </p>
      <p v-else>
        {{ $t("prompts.deleteMessageMultiple", { count: selectedCount }) }}
      </p>
      <template v-if="enableTrash">
        <p class="trash-note" v-if="!permanent">
          {{ $t("trash.movedNote") }}
        </p>
        <label class="trash-permanent">
          <input type="checkbox" v-model="permanent" />
          {{ $t("trash.permanent") }}
        </label>
      </template>
    </div>
    <div class="card-action">
      <button
        @click="closeHovers"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
        tabindex="2"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        id="focus-prompt"
        @click="submit"
        class="button button--flat button--red"
        :aria-label="$t('buttons.delete')"
        :title="$t('buttons.delete')"
        tabindex="1"
      >
        {{ $t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions, mapState, mapWritableState } from "pinia";
import { files as api } from "@/api";
import buttons from "@/utils/buttons";
import { useFileStore } from "@/stores/file";
import { enableTrash } from "@/utils/constants";
import { useLayoutStore } from "@/stores/layout";

export default {
  name: "delete",
  inject: ["$showError", "$showSuccess"],
  data() {
    return { permanent: false };
  },
  computed: {
    enableTrash: () => enableTrash,
    ...mapState(useFileStore, [
      "isListing",
      "selectedCount",
      "req",
      "selected",
    ]),
    ...mapState(useLayoutStore, ["currentPrompt"]),
    ...mapWritableState(useFileStore, ["reload", "preselect"]),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    // With the trash on, an item answered "deleted" was too big to keep.
    noteTooLarge(responses) {
      if (
        enableTrash &&
        !this.permanent &&
        responses.some((r) => r?.headers?.get("X-Trash") === "deleted")
      ) {
        this.$showSuccess(this.$t("trash.tooLarge"));
      }
    },
    submit: async function () {
      buttons.loading("delete");

      try {
        if (!this.isListing) {
          const res = await api.remove(this.$route.path, this.permanent);
          this.noteTooLarge([res]);
          buttons.success("delete");

          this.currentPrompt?.confirm();
          this.closeHovers();
          return;
        }

        this.closeHovers();

        if (this.selectedCount === 0) {
          return;
        }

        const promises = [];
        for (const index of this.selected) {
          promises.push(api.remove(this.req.items[index].url, this.permanent));
        }

        this.noteTooLarge(await Promise.all(promises));
        buttons.success("delete");

        const nearbyItem =
          this.req.items[Math.max(0, Math.min(this.selected) - 1)];

        this.preselect = nearbyItem?.path;

        this.reload = true;
      } catch (e) {
        buttons.done("delete");
        this.$showError(e);
        if (this.isListing) this.reload = true;
      }
    },
  },
};
</script>

<style scoped>
.trash-note {
  font-size: 0.9em;
  opacity: 0.75;
}

.trash-permanent {
  display: block;
  margin-top: 0.4em;
}
</style>
