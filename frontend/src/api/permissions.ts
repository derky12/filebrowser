import { fetchJSON } from "./utils";
import { encodePath } from "@/utils/url";

export function get(path: string) {
  return fetchJSON<PermissionsInfo>(`/api/permissions${encodePath(path)}`);
}

export function set(path: string, change: PermissionsChange) {
  return fetchJSON<PermissionsResult>(`/api/permissions${encodePath(path)}`, {
    method: "PATCH",
    body: JSON.stringify(change),
  });
}
