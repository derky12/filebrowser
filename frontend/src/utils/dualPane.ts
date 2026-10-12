// Pure helpers of the dual pane view.

export type SortKey = "name" | "size" | "modified";

export interface PaneEntry {
  name: string;
  isDir: boolean;
  size: number;
  modified: string;
}

// Folders first, then by the chosen key.
export function sortEntries<T extends PaneEntry>(
  entries: T[],
  key: SortKey,
  ascending: boolean
): T[] {
  const sign = ascending ? 1 : -1;
  return [...entries].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;

    let order = 0;
    if (key === "size") order = a.size - b.size;
    else if (key === "modified")
      order = Date.parse(a.modified) - Date.parse(b.modified);
    if (order === 0) {
      order = a.name.localeCompare(b.name, undefined, {
        numeric: true,
        sensitivity: "base",
      });
    }

    return order * sign;
  });
}

// The folders leading to a path: "/a/b" -> [{name: "a", path: "/a"}, {name: "b", path: "/a/b"}].
export function crumbsOf(path: string) {
  const parts = path.split("/").filter(Boolean);
  return parts.map((name, i) => ({
    name,
    path: "/" + parts.slice(0, i + 1).join("/"),
  }));
}

export const parentOf = (path: string) => {
  const parts = path.split("/").filter(Boolean);
  parts.pop();
  return "/" + parts.join("/");
};

export const joinPath = (folder: string, name: string) =>
  (folder === "/" ? "" : folder) + "/" + name;

// Selection after a click: a plain click selects one item, Ctrl/Cmd toggles
// it and Shift selects the range from the anchor.
export function clickSelection(
  names: string[],
  selected: ReadonlySet<string>,
  anchor: string | null,
  clicked: string,
  mods: { toggle: boolean; range: boolean }
): { selected: Set<string>; anchor: string } {
  if (mods.range && anchor !== null && names.includes(anchor)) {
    const from = names.indexOf(anchor);
    const to = names.indexOf(clicked);
    const [lo, hi] = from < to ? [from, to] : [to, from];
    return { selected: new Set(names.slice(lo, hi + 1)), anchor };
  }

  if (mods.toggle) {
    const next = new Set(selected);
    if (next.has(clicked)) next.delete(clicked);
    else next.add(clicked);
    return { selected: next, anchor: clicked };
  }

  return { selected: new Set([clicked]), anchor: clicked };
}
