import { fetchJSON } from "./utils";

export interface ArchiveResult {
  path: string;
  files: number;
  size: number;
  skipped: number;
  errors: string[];
}

const post = (body: unknown) =>
  fetchJSON<ArchiveResult>("/api/archive", {
    method: "POST",
    body: JSON.stringify(body),
  });

export const compress = (
  paths: string[],
  destination: string,
  format: string
) => post({ action: "compress", paths, destination, format });

export const extract = (path: string, destination: string, overwrite = false) =>
  post({ action: "extract", path, destination, overwrite });
