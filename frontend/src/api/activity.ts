import { fetchJSON } from "./utils";

export function list(params: {
  user?: string;
  action?: string;
  q?: string;
  limit?: number;
  offset?: number;
}) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") query.set(key, String(value));
  }

  return fetchJSON<ActivityPage>(`/api/activity?${query}`);
}
