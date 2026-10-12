// Completing a folder path while it is typed: "/docs/ph" suggests the folders
// of /docs that start with "ph".

export interface SuggestEntry {
  name: string;
  isDir: boolean;
}

// Splits what was typed into the folder to list and the start of the name
// being typed: "/a/b/c" -> {parent: "/a/b", prefix: "c"}, "/a/b/" -> {parent:
// "/a/b", prefix: ""}. A path without a leading slash is taken from the root.
export function splitTyped(typed: string): { parent: string; prefix: string } {
  const path = typed.startsWith("/") ? typed : "/" + typed;
  const cut = path.lastIndexOf("/");
  const parent = path.slice(0, cut).replace(/\/+$/, "") || "/";

  return { parent, prefix: path.slice(cut + 1) };
}

// The folders whose name starts with the prefix, case-insensitively, in natural
// order. Names starting with a dot are only offered when the prefix does too.
export function suggestFolders(
  entries: SuggestEntry[],
  prefix: string,
  limit = 12
): string[] {
  const wanted = prefix.toLowerCase();
  const showDotted = prefix.startsWith(".");

  return entries
    .filter(
      (e) =>
        e.isDir &&
        (showDotted || !e.name.startsWith(".")) &&
        e.name.toLowerCase().startsWith(wanted)
    )
    .map((e) => e.name)
    .sort((a, b) =>
      a.localeCompare(b, undefined, { numeric: true, sensitivity: "base" })
    )
    .slice(0, limit);
}

// The path after choosing a folder: it ends with a slash, ready for the next level.
export function chooseFolder(parent: string, name: string): string {
  return (parent === "/" ? "" : parent) + "/" + name + "/";
}
