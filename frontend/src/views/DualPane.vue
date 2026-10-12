<template>
  <div id="dual-pane">
    <header-bar showMenu showLogo>
      <title>{{ t("dual.title") }}</title>
      <template #actions>
        <action
          icon="arrow_forward"
          :label="t('dual.copyOver')"
          @action="transferActive(true)"
        />
        <action
          icon="drive_file_move"
          :label="t('dual.moveOver')"
          @action="transferActive(false)"
        />
        <action
          icon="create_new_folder"
          :label="t('dual.newFolder')"
          @action="newFolder"
        />
        <action
          icon="delete"
          :label="t('buttons.delete')"
          @action="removeSelected"
        />
        <action icon="swap_horiz" :label="t('dual.swap')" @action="swapPanes" />
        <action
          icon="refresh"
          :label="t('dual.refresh')"
          @action="reloadBoth"
        />
      </template>
    </header-bar>

    <p class="dual-help">{{ t("dual.help") }}</p>

    <div class="dual-body">
      <section
        v-for="side in SIDES"
        :key="side"
        class="pane"
        :class="{ active: active === side, over: dropOver === side }"
        @click="active = side"
        @dragover="paneDragOver(side, $event)"
        @dragleave="dropOver = null"
        @drop="paneDrop(side, $event)"
      >
        <div class="pane-bar">
          <button
            class="action"
            :title="t('dual.up')"
            :aria-label="t('dual.up')"
            :disabled="panes[side].path === '/'"
            @click.stop="go(side, parentOf(panes[side].path))"
          >
            <i class="material-icons">arrow_upward</i>
          </button>
          <div
            class="pane-crumbs"
            role="navigation"
            :aria-label="t('dual.path')"
          >
            <a @click.stop="go(side, '/')">/</a>
            <template
              v-for="crumb in crumbsOf(panes[side].path)"
              :key="crumb.path"
            >
              <span>›</span>
              <a @click.stop="go(side, crumb.path)">{{ crumb.name }}</a>
            </template>
          </div>
          <span class="pane-count">{{
            t("dual.selected", { n: panes[side].selected.size })
          }}</span>
        </div>

        <div class="pane-list" role="listbox" :aria-label="side">
          <div class="pane-row head">
            <span class="name" @click.stop="sortBy(side, 'name')">{{
              t("dual.name")
            }}</span>
            <span class="size" @click.stop="sortBy(side, 'size')">{{
              t("dual.size")
            }}</span>
            <span class="modified" @click.stop="sortBy(side, 'modified')">{{
              t("dual.modified")
            }}</span>
          </div>
          <p v-if="panes[side].loading" class="pane-note">
            {{ t("files.loading") }}
          </p>
          <p v-else-if="panes[side].entries.length === 0" class="pane-note">
            {{ t("dual.empty") }}
          </p>
          <div
            v-for="item in panes[side].entries"
            :key="item.name"
            class="pane-row"
            :class="{
              selected: panes[side].selected.has(item.name),
              dir: item.isDir,
              target: dropFolder === side + '/' + item.name,
            }"
            role="option"
            :aria-selected="panes[side].selected.has(item.name)"
            :data-name="item.name"
            draggable="true"
            @click.stop="click(side, item, $event)"
            @dblclick.stop="open(side, item)"
            @dragstart="dragStart(side, item, $event)"
            @dragover="rowDragOver(side, item, $event)"
            @dragleave="dropFolder = null"
            @drop="rowDrop(side, item, $event)"
          >
            <span class="name">
              <i class="material-icons">{{
                item.isDir ? "folder" : "insert_drive_file"
              }}</i>
              {{ item.name }}
            </span>
            <span class="size">{{
              item.isDir ? "—" : filesize(item.size)
            }}</span>
            <span class="modified">{{ dayjs(item.modified).fromNow() }}</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRoute } from "vue-router";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import { files as api } from "@/api";
import { filesize } from "@/utils";
import { encodePath } from "@/utils/url";
import { transferItems } from "@/utils/drop";
import type { TransferSource } from "@/utils/drop";
import {
  clickSelection,
  crumbsOf,
  joinPath,
  parentOf,
  sortEntries,
} from "@/utils/dualPane";
import type { SortKey } from "@/utils/dualPane";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";

type Side = "left" | "right";
const SIDES: Side[] = ["left", "right"];
const STORAGE_KEY = "dualPanePaths";
const DRAG_TYPE = "application/x-filebrowser-pane";

interface Pane {
  path: string;
  entries: ResourceItem[];
  selected: Set<string>;
  anchor: string | null;
  loading: boolean;
  sortKey: SortKey;
  ascending: boolean;
}

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;
const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const newPane = (path: string): Pane => ({
  path,
  entries: [],
  selected: new Set(),
  anchor: null,
  loading: false,
  sortKey: "name",
  ascending: true,
});

const stored = (() => {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "{}");
  } catch {
    return {};
  }
})();
const fromQuery = (name: string) => {
  const value = route.query[name];
  return typeof value === "string" && value.startsWith("/") ? value : undefined;
};

const panes = reactive<Record<Side, Pane>>({
  left: newPane(fromQuery("left") ?? stored.left ?? "/"),
  right: newPane(fromQuery("right") ?? stored.right ?? "/"),
});
const active = ref<Side>("left");
const dropOver = ref<Side | null>(null);
const dropFolder = ref<string | null>(null);
const other = (side: Side): Side => (side === "left" ? "right" : "left");

// The (encoded) listing url of a path, with a trailing slash.
const urlOf = (path: string) =>
  `/files${encodePath(path === "/" ? "" : path)}/`;

const remember = () =>
  localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({ left: panes.left.path, right: panes.right.path })
  );

const load = async (side: Side) => {
  const pane = panes[side];
  pane.loading = true;
  try {
    const resource = await api.fetch(urlOf(pane.path));
    pane.entries = sortEntries(
      resource.items ?? [],
      pane.sortKey,
      pane.ascending
    );
    // Forget selected names that are gone.
    const names = new Set(pane.entries.map((e) => e.name));
    pane.selected = new Set([...pane.selected].filter((n) => names.has(n)));
  } catch (e) {
    $showError(e as Error);
    // A folder that cannot be read (removed, no access): fall back to the root.
    if (pane.path !== "/") {
      pane.path = "/";
      await load(side);
    }
  } finally {
    pane.loading = false;
  }
};

const go = async (side: Side, path: string) => {
  panes[side].path = path;
  panes[side].selected = new Set();
  panes[side].anchor = null;
  remember();
  await load(side);
};

const reloadBoth = () => Promise.all(SIDES.map(load));

const swapPanes = async () => {
  const left = panes.left.path;
  panes.left.path = panes.right.path;
  panes.right.path = left;
  panes.left.selected = new Set();
  panes.right.selected = new Set();
  remember();
  await reloadBoth();
};

const sortBy = (side: Side, key: SortKey) => {
  const pane = panes[side];
  pane.ascending = pane.sortKey === key ? !pane.ascending : true;
  pane.sortKey = key;
  pane.entries = sortEntries(pane.entries, key, pane.ascending);
};

const click = (side: Side, item: ResourceItem, event: MouseEvent) => {
  active.value = side;
  const pane = panes[side];
  const result = clickSelection(
    pane.entries.map((e) => e.name),
    pane.selected,
    pane.anchor,
    item.name,
    { toggle: event.ctrlKey || event.metaKey, range: event.shiftKey }
  );
  pane.selected = result.selected;
  pane.anchor = result.anchor;
};

// A folder is entered; a file is opened in the file browser.
const open = (side: Side, item: ResourceItem) => {
  const path = joinPath(panes[side].path, item.name);
  if (item.isDir) go(side, path);
  else router.push(`/files${encodePath(path)}`);
};

const sourcesOf = (side: Side, names?: string[]): TransferSource[] => {
  const pane = panes[side];
  const wanted = new Set(names ?? pane.selected);
  return pane.entries
    .filter((e) => wanted.has(e.name))
    .map((e) => ({
      from: e.url,
      name: e.name,
      size: e.size,
      isDir: e.isDir,
      modified: e.modified,
    }));
};

const transfer = async (
  from: Side,
  names: string[] | undefined,
  targetUrl: string,
  copy: boolean
) => {
  await transferItems(
    sourcesOf(from, names),
    targetUrl,
    copy,
    $showError,
    () => {
      reloadBoth();
    }
  );
};

// The buttons and F5/F6 act from the active pane on the other one.
const transferActive = async (copy: boolean) => {
  const from = active.value;
  if (panes[from].selected.size === 0) {
    $showError(t("dual.nothingSelected"));
    return;
  }
  await transfer(from, undefined, urlOf(panes[other(from)].path), copy);
};

const newFolder = async () => {
  const name = window.prompt(t("dual.folderName"))?.trim();
  if (!name) return;

  try {
    await api.post(
      `${urlOf(panes[active.value].path)}${encodeURIComponent(name)}/`
    );
    await load(active.value);
  } catch (e) {
    $showError(e as Error);
  }
};

const removeSelected = async () => {
  const side = active.value;
  const sources = sourcesOf(side);
  if (sources.length === 0) {
    $showError(t("dual.nothingSelected"));
    return;
  }
  if (!window.confirm(t("dual.confirmDelete", { n: sources.length }))) return;

  try {
    await Promise.all(sources.map((s) => api.remove(s.from)));
    $showSuccess(t("dual.deleted", { n: sources.length }));
  } catch (e) {
    $showError(e as Error);
  }
  await reloadBoth();
};

// ---- drag and drop between the panes (Ctrl/Cmd copies, otherwise it moves)

const dragStart = (side: Side, item: ResourceItem, event: DragEvent) => {
  const pane = panes[side];
  if (!pane.selected.has(item.name)) {
    pane.selected = new Set([item.name]);
    pane.anchor = item.name;
  }
  active.value = side;

  event.dataTransfer?.setData(
    DRAG_TYPE,
    JSON.stringify({ side, names: [...pane.selected] })
  );
  if (event.dataTransfer) event.dataTransfer.effectAllowed = "copyMove";
};

const dragged = (event: DragEvent): { side: Side; names: string[] } | null => {
  const raw = event.dataTransfer?.getData(DRAG_TYPE);
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
};

const isOurDrag = (event: DragEvent) =>
  !!event.dataTransfer?.types.includes(DRAG_TYPE);

const dropEffect = (event: DragEvent) =>
  event.ctrlKey || event.metaKey ? "copy" : "move";

const paneDragOver = (side: Side, event: DragEvent) => {
  if (!isOurDrag(event)) return;
  event.preventDefault();
  if (event.dataTransfer) event.dataTransfer.dropEffect = dropEffect(event);
  dropOver.value = side;
};

const paneDrop = async (side: Side, event: DragEvent) => {
  dropOver.value = null;
  dropFolder.value = null;
  const info = dragged(event);
  if (!info) return;
  event.preventDefault();

  // Dropping on the folder the items already are in does nothing.
  if (info.side === side) return;
  await transfer(
    info.side,
    info.names,
    urlOf(panes[side].path),
    dropEffect(event) === "copy"
  );
};

const rowDragOver = (side: Side, item: ResourceItem, event: DragEvent) => {
  if (!item.isDir || !isOurDrag(event)) return;
  event.preventDefault();
  event.stopPropagation();
  if (event.dataTransfer) event.dataTransfer.dropEffect = dropEffect(event);
  dropFolder.value = `${side}/${item.name}`;
};

const rowDrop = async (side: Side, item: ResourceItem, event: DragEvent) => {
  dropOver.value = null;
  dropFolder.value = null;
  if (!item.isDir) return;
  const info = dragged(event);
  if (!info) return;
  event.preventDefault();
  event.stopPropagation();

  // A folder cannot be dropped onto itself.
  const names =
    info.side === side ? info.names.filter((n) => n !== item.name) : info.names;
  if (names.length === 0) return;
  await transfer(
    info.side,
    names,
    urlOf(joinPath(panes[side].path, item.name)),
    dropEffect(event) === "copy"
  );
};

// ---- keyboard

const keyEvent = (event: KeyboardEvent) => {
  const target = event.target as HTMLElement | null;
  if (target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName))
    return;
  if (event.altKey) return;

  const pane = panes[active.value];
  switch (event.key) {
    case "Tab":
      event.preventDefault();
      active.value = other(active.value);
      break;
    case "F5":
      event.preventDefault();
      transferActive(true);
      break;
    case "F6":
      event.preventDefault();
      transferActive(false);
      break;
    case "F7":
      event.preventDefault();
      newFolder();
      break;
    case "Delete":
      event.preventDefault();
      removeSelected();
      break;
    case "Backspace":
      event.preventDefault();
      go(active.value, parentOf(pane.path));
      break;
    case "Enter": {
      const first = pane.entries.find((e) => pane.selected.has(e.name));
      if (first) open(active.value, first);
      break;
    }
    case "a":
    case "A":
      if (event.ctrlKey || event.metaKey) {
        event.preventDefault();
        pane.selected = new Set(pane.entries.map((e) => e.name));
      }
      break;
    case "ArrowDown":
    case "ArrowUp": {
      event.preventDefault();
      const names = pane.entries.map((e) => e.name);
      if (names.length === 0) break;
      const current = pane.anchor ? names.indexOf(pane.anchor) : -1;
      const next = Math.max(
        0,
        Math.min(
          names.length - 1,
          current + (event.key === "ArrowDown" ? 1 : -1)
        )
      );
      pane.selected = new Set([names[next]]);
      pane.anchor = names[next];
      break;
    }
  }
};

onMounted(() => {
  window.addEventListener("keydown", keyEvent);
  reloadBoth();
});
onBeforeUnmount(() => window.removeEventListener("keydown", keyEvent));
</script>

<style scoped>
.dual-help {
  margin: 0.4em 1em;
  font-size: 0.85em;
  opacity: 0.65;
}

.dual-body {
  display: flex;
  gap: 0.8em;
  padding: 0 1em 1em;
  height: calc(100vh - 9em);
  min-height: 20em;
}

.pane {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  border: 2px solid var(--divider);
  border-radius: 6px;
  background: var(--surfacePrimary);
}

.pane.active {
  border-color: var(--blue);
}

.pane.over {
  background: var(--hover);
}

.pane-bar {
  display: flex;
  align-items: center;
  gap: 0.4em;
  padding: 0.2em 0.4em;
  border-bottom: 1px solid var(--divider);
}

.pane-bar .action {
  padding: 0.2em;
}

.pane-crumbs {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-crumbs a {
  cursor: pointer;
}

.pane-crumbs span {
  margin: 0 0.3em;
  opacity: 0.6;
}

.pane-count {
  font-size: 0.8em;
  opacity: 0.6;
  white-space: nowrap;
}

.pane-list {
  flex: 1;
  overflow: auto;
}

.pane-row {
  display: flex;
  align-items: center;
  gap: 0.8em;
  padding: 0.3em 0.6em;
  cursor: default;
  user-select: none;
}

.pane-row.head {
  position: sticky;
  top: 0;
  z-index: 1;
  font-size: 0.8em;
  font-weight: bold;
  background: var(--surfacePrimary);
  border-bottom: 1px solid var(--divider);
}

.pane-row.head span {
  cursor: pointer;
}

.pane-row .name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-row .name i {
  vertical-align: middle;
  margin-right: 0.3em;
  color: var(--icon-blue, #2196f3);
}

.pane-row:not(.dir):not(.head) .name i {
  color: var(--iconSecondary, #8a8a8a);
}

.pane-row .size {
  width: 6em;
  text-align: right;
  font-size: 0.9em;
}

.pane-row .modified {
  width: 8em;
  font-size: 0.85em;
  opacity: 0.7;
}

.pane-row:not(.head):hover {
  background: var(--hover);
}

.pane-row.selected {
  background: var(--blue);
  color: #fff;
}

.pane-row.selected .name i {
  color: #fff;
}

.pane-row.target {
  outline: 2px dashed var(--blue);
  outline-offset: -2px;
}

.pane-note {
  padding: 1em;
  opacity: 0.6;
}

@media (max-width: 900px) {
  .dual-body {
    flex-direction: column;
    height: auto;
  }

  .pane {
    height: 22em;
  }

  .pane-row .modified {
    display: none;
  }
}
</style>
