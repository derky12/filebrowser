import { defineStore } from "pinia";
import { sync as api } from "@/api";
import { folderKind } from "@/utils/syncState";
import type { SyncKind } from "@/utils/syncState";

// The synced folders, polled while the file list is open, so that folders can
// show a badge.
export const useSyncStore = defineStore("sync", {
  state: (): { kinds: Record<string, SyncKind>; timer: number | null } => ({
    kinds: {},
    timer: null,
  }),
  actions: {
    async load() {
      try {
        const status = await api.status();
        const kinds: Record<string, SyncKind> = {};
        for (const folder of status.folders) {
          if (!folder.external) kinds[folder.path] = folderKind(folder);
        }
        this.kinds = kinds;
      } catch {
        // Not allowed or Syncthing is down: no badges.
        this.kinds = {};
      }
    },
    start() {
      if (this.timer !== null) return;
      this.load();
      this.timer = window.setInterval(() => this.load(), 8000);
    },
    stop() {
      if (this.timer !== null) window.clearInterval(this.timer);
      this.timer = null;
    },
  },
});
