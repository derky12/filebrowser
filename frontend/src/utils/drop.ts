import { files as api } from "@/api";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import * as upload from "@/utils/upload";

export interface TransferSource {
  from: string; // the (encoded) url of the item
  name: string;
  size: number;
  isDir: boolean;
  modified: string;
}

// Moves (or copies) items into the folder at `targetUrl` (a listing url with a
// trailing slash), asking what to do about names that already exist there.
// `onDone` runs after the transfer went through.
export async function transferItems(
  sources: TransferSource[],
  targetUrl: string,
  copy: boolean,
  showError: IToastError,
  onDone: () => void = () => {}
): Promise<void> {
  const layoutStore = useLayoutStore();

  if (sources.length === 0) return;

  const items: any[] = sources.map((source) => ({
    from: source.from,
    to: targetUrl + encodeURIComponent(source.name),
    name: source.name,
    size: source.size,
    isDir: source.isDir,
    modified: source.modified,
    overwrite: false,
    rename: false,
  }));

  const action = (overwrite?: boolean, rename?: boolean) => {
    (copy ? api.copy : api.move)(items, overwrite, rename)
      .then(onDone)
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

// Moves (or copies, when Ctrl/Cmd is held) the items selected in the file list
// into the folder at `targetUrl`.
export async function dropSelectedInto(
  targetUrl: string,
  event: Event,
  showError: IToastError
): Promise<void> {
  const fileStore = useFileStore();

  if (fileStore.selectedCount === 0 || !fileStore.req) return;

  const sources: TransferSource[] = fileStore.selected.map((i) => {
    const item = fileStore.req!.items[i];
    return {
      from: item.url,
      name: item.name,
      size: item.size,
      isDir: item.isDir,
      modified: item.modified,
    };
  });

  const copy =
    (event as KeyboardEvent).ctrlKey || (event as KeyboardEvent).metaKey;

  await transferItems(sources, targetUrl, copy, showError, () => {
    fileStore.reload = true;
  });
}
