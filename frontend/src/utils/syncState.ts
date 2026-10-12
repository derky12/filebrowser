// How a synced folder is doing, as one of four kinds, shared by the Sync page
// and the badge on folders in the file list.
export type SyncKind = "ok" | "busy" | "off" | "err";

export const minCompletion = (folder: SyncFolder) =>
  folder.remotes.length === 0
    ? 100
    : Math.min(...folder.remotes.map((r) => r.completion));

export function folderKind(folder: SyncFolder): SyncKind {
  if (folder.error) return "err";
  if (folder.paused) return "off";
  if (folder.remotes.length > 0 && folder.remotes.every((r) => !r.connected))
    return "off";
  if (folder.state === "idle" && minCompletion(folder) >= 100) return "ok";
  return "busy";
}
