interface DuEntry {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  files: number;
  complete: boolean;
}

interface DuResponse {
  path: string;
  total: number;
  files: number;
  entries: DuEntry[];
  partial: boolean;
}
