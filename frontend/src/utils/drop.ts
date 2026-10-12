import { files as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import * as upload from "@/utils/upload";

// Moves (or copies, when Ctrl/Cmd is held) the currently selected listing
// items into the folder at `targetUrl` (a listing url with trailing slash).
export async function dropSelectedInto(
  targetUrl: string,
  event: Event,
  showError: IToastError
): Promise<void> {
  const fileStore = useFileStore();
  const layoutStore = useLayoutStore();

  if (fileStore.selectedCount === 0 || !fileStore.req) return;

  const items: any[] = [];

  for (const i of fileStore.selected) {
    const item = fileStore.req.items[i];
    items.push({
      from: item.url,
      to: targetUrl + encodeURIComponent(item.name),
      name: item.name,
      size: item.size,
      isDir: item.isDir,
      modified: item.modified,
      overwrite: false,
      rename: false,
    });
  }

  const copy =
    (event as KeyboardEvent).ctrlKey || (event as KeyboardEvent).metaKey;

  const action = (overwrite?: boolean, rename?: boolean) => {
    (copy ? api.copy : api.move)(items, overwrite, rename)
      .then(() => {
        fileStore.reload = true;
      })
      .catch(showError);
  };

  const conflict = await upload.checkConflict(items, targetUrl, true);

  if (conflict.length > 0) {
    layoutStore.showHover({
      prompt: "resolve-conflict",
      props: {
        conflict: conflict,
      },
      confirm: (event: Event, result: Array<ConflictingResource>) => {
        event.preventDefault();
        layoutStore.closeHovers();
        for (let i = result.length - 1; i >= 0; i--) {
          const item = result[i];
          if (item.checked.length == 2) {
            items[item.index].rename = true;
          } else if (item.checked.length == 1 && item.checked[0] == "origin") {
            items[item.index].overwrite = true;
          } else {
            items.splice(item.index, 1);
          }
        }
        if (items.length > 0) {
          action();
        }
      },
    });

    return;
  }

  action(false, false);
}
