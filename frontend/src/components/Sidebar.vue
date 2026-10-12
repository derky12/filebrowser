<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <button @click="toAccountSettings" class="action">
        <i class="material-icons">person</i>
        <span>{{ user.username }}</span>
      </button>
      <button
        class="action"
        @click="toRoot"
        :aria-label="$t('sidebar.myFiles')"
        :title="$t('sidebar.myFiles')"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.myFiles") }}</span>
      </button>

      <button
        v-if="enableTrash && user.perm.delete"
        class="action"
        @click="toTrash"
        :aria-label="$t('trash.title')"
        :title="$t('trash.title')"
      >
        <i class="material-icons">delete_outline</i>
        <span>{{ $t("trash.title") }}</span>
      </button>

      <div v-if="user.favorites && user.favorites.length > 0" class="favorites">
        <p class="favorites-title">{{ $t("sidebar.favorites") }}</p>
        <div
          v-for="favorite in user.favorites"
          :key="favorite"
          class="action favorite-item"
          :class="{ 'drag-over': dragOverFavorite === favorite }"
          role="button"
          tabindex="0"
          :title="favorite"
          :aria-label="favorite"
          @click="openFavorite(favorite)"
          @keydown.enter="openFavorite(favorite)"
          @dragover="favoriteDragOver($event, favorite)"
          @dragleave="dragOverFavorite = null"
          @drop="favoriteDrop($event, favorite)"
        >
          <i class="material-icons" :style="{ color: folderColor(favorite) }"
            >folder_special</i
          >
          <span>{{ favoriteName(favorite) }}</span>
          <button
            class="favorite-remove"
            :title="$t('buttons.unfavorite')"
            :aria-label="$t('buttons.unfavorite')"
            @click.stop="removeFavorite(favorite)"
          >
            <i class="material-icons">close</i>
          </button>
        </div>
      </div>

      <div v-if="user.perm.create">
        <button
          @click="showHover('newDir')"
          class="action"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>

        <button
          @click="showHover('newFile')"
          class="action"
          :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')"
        >
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
      </div>

      <div v-if="user.perm.admin">
        <button
          class="action"
          @click="toGlobalSettings"
          :aria-label="$t('sidebar.settings')"
          :title="$t('sidebar.settings')"
        >
          <i class="material-icons">settings_applications</i>
          <span>{{ $t("sidebar.settings") }}</span>
        </button>
      </div>
      <button
        v-if="canLogout"
        @click="logout"
        class="action"
        id="logout"
        :aria-label="$t('sidebar.logout')"
        :title="$t('sidebar.logout')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.logout") }}</span>
      </button>
    </template>
    <template v-else>
      <router-link
        v-if="!hideLoginButton"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.login')"
        :title="$t('sidebar.login')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>

      <router-link
        v-if="signup"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')"
      >
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </template>

    <div
      class="credits"
      v-if="isFiles && !disableUsedPercentage"
      style="width: 90%; margin: 2em 2.5em 3em 2.5em"
    >
      <progress-bar :val="usage.usedPercentage" size="small"></progress-bar>
      <br />
      {{ $t("sidebar.diskUsed", { used: usage.used, total: usage.total }) }}
    </div>

    <p class="credits">
      <span>
        <span v-if="disableExternal">File Browser</span>
        <a
          v-else
          rel="noopener noreferrer"
          target="_blank"
          href="https://github.com/filebrowser/filebrowser"
          >File Browser</a
        >
        <span> {{ " " }} {{ version }}</span>
      </span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </p>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  version,
  signup,
  hideLoginButton,
  disableExternal,
  disableUsedPercentage,
  enableTrash,
  noAuth,
  logoutPage,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import { dropSelectedInto } from "@/utils/drop";
import { favoriteName, favoriteToUrl, urlToFavorite } from "@/utils/favorites";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    return {
      usage,
      usageAbortController: new AbortController(),
      favoriteName,
    };
  },
  components: {
    ProgressBar,
  },
  data() {
    return { dragOverFavorite: null };
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload", "selectedCount"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    signup: () => signup,
    enableTrash: () => enableTrash,
    hideLoginButton: () => hideLoginButton,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && (loginPage || logoutPage !== "/login"),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    ...mapActions(useAuthStore, ["toggleFavorite"]),
    abortOngoingFetchUsage() {
      this.usageAbortController.abort();
    },
    async fetchUsage() {
      const path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        this.abortOngoingFetchUsage();
        this.usageAbortController = new AbortController();
        const usage = await api.usage(path, this.usageAbortController.signal);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } finally {
        return Object.assign(this.usage, usageStats);
      }
    },
    folderColor(favorite) {
      return this.user.folderColors?.[favorite];
    },
    openFavorite(favorite) {
      this.$router.push({ path: favoriteToUrl(favorite) });
      this.closeHovers();
    },
    removeFavorite(favorite) {
      this.toggleFavorite(favorite).catch(this.$showError);
    },
    // A favorite accepts the current selection unless it is the folder the
    // selection already lives in.
    canDropOn(favorite) {
      return (
        this.selectedCount > 0 && urlToFavorite(this.$route.path) !== favorite
      );
    },
    favoriteDragOver(event, favorite) {
      if (!this.canDropOn(favorite)) return;

      event.preventDefault();
      event.dataTransfer.dropEffect =
        event.ctrlKey || event.metaKey ? "copy" : "move";
      this.dragOverFavorite = favorite;
    },
    async favoriteDrop(event, favorite) {
      this.dragOverFavorite = null;
      if (!this.canDropOn(favorite)) return;

      event.preventDefault();
      await dropSelectedInto(favoriteToUrl(favorite), event, this.$showError);
    },
    toTrash() {
      this.$router.push({ path: "/trash" });
      this.closeHovers();
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    toAccountSettings() {
      this.$router.push({ path: "/settings/profile" });
      this.closeHovers();
    },
    toGlobalSettings() {
      this.$router.push({ path: "/settings/global" });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
    logout: auth.logout,
  },
  watch: {
    $route: {
      handler(to) {
        if (to.path.includes("/files")) {
          this.fetchUsage();
        }
      },
      immediate: true,
    },
  },
  unmounted() {
    this.abortOngoingFetchUsage();
  },
};
</script>

<style scoped>
.favorites-title {
  margin: 0.6em 0.8em 0.2em;
  font-size: 0.8em;
  text-transform: uppercase;
  opacity: 0.6;
}

.favorite-item {
  position: relative;
  cursor: pointer;
  user-select: none;
}

.favorite-item.drag-over {
  background: var(--hover);
  outline: 2px dashed var(--blue);
  outline-offset: -2px;
}

.favorite-remove {
  position: absolute;
  right: 0.3em;
  top: 50%;
  transform: translateY(-50%);
  display: none;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  padding: 0;
}

.favorite-remove .material-icons {
  font-size: 1.1em;
}

.favorite-item:hover .favorite-remove,
.favorite-item:focus-within .favorite-remove {
  display: block;
}

@media (hover: none) {
  .favorite-remove {
    display: block;
  }
}
</style>
