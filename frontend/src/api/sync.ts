import { fetchJSON, fetchURL } from "./utils";

const json = (body: unknown): ApiOpts => ({
  method: "POST",
  body: JSON.stringify(body),
});

export function status() {
  return fetchJSON<SyncStatus>("/api/sync");
}

export async function addDevice(device: {
  deviceID: string;
  name: string;
  address?: string;
}) {
  await fetchURL("/api/sync/devices", json(device));
}

export async function removeDevice(id: string) {
  await fetchURL(`/api/sync/devices/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function createFolder(folder: {
  path: string;
  deviceIDs: string[];
  mode: "oneway" | "twoway";
  versioning: boolean;
}) {
  return fetchJSON<{ id: string }>("/api/sync/folders", json(folder));
}

export async function setPaused(id: string, paused: boolean) {
  await fetchURL(`/api/sync/folders/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ paused }),
  });
}

export async function removeFolder(id: string) {
  await fetchURL(`/api/sync/folders/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function dismissPendingDevice(id: string) {
  await fetchURL(`/api/sync/pending/devices/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function dismissPendingFolder(folder: string, device: string) {
  const query = new URLSearchParams({ folder, device });
  await fetchURL(`/api/sync/pending/folders?${query}`, { method: "DELETE" });
}

export async function acceptFolder(accept: {
  folderID: string;
  deviceID: string;
  path: string;
  mode: "receive" | "twoway";
}) {
  await fetchURL("/api/sync/pending/folders/accept", json(accept));
}
