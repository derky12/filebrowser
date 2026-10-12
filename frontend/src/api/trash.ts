import { fetchJSON, fetchURL } from "./utils";

export function list() {
  return fetchJSON<TrashListing>("/api/trash");
}

export function restore(id: string, rename = false) {
  return fetchJSON<{ path: string }>(
    `/api/trash/${encodeURIComponent(id)}/restore${rename ? "?rename=true" : ""}`,
    { method: "POST" }
  );
}

export async function remove(id: string) {
  await fetchURL(`/api/trash/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export async function empty() {
  await fetchURL("/api/trash", { method: "DELETE" });
}
