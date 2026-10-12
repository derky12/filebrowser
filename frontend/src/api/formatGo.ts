import { fetchURL } from "./utils";

// Formats Go source on the server (gofmt).
export default async function formatGo(source: string): Promise<string> {
  const res = await fetchURL("/api/format/go", {
    method: "POST",
    body: source,
  });

  return res.text();
}
