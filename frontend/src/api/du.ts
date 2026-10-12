import { fetchJSON } from "./utils";
import { encodePath } from "@/utils/url";

export function usage(path: string, signal?: AbortSignal) {
  return fetchJSON<DuResponse>(`/api/du${encodePath(path)}`, { signal });
}
