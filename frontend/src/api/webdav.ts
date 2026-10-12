import { fetchJSON, fetchURL } from "./utils";

export function info() {
  return fetchJSON<WebdavInfo>("/api/webdav");
}

export function createShare(share: {
  path: string;
  name?: string;
  readOnly: boolean;
}) {
  return fetchJSON<WebdavShare>("/api/webdav/shares", {
    method: "POST",
    body: JSON.stringify(share),
  });
}

export function setReadOnly(name: string, readOnly: boolean) {
  return fetchJSON<WebdavShare>(
    `/api/webdav/shares/${encodeURIComponent(name)}`,
    { method: "PATCH", body: JSON.stringify({ readOnly }) }
  );
}

export async function removeShare(name: string) {
  await fetchURL(`/api/webdav/shares/${encodeURIComponent(name)}`, {
    method: "DELETE",
  });
}
