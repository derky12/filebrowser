interface TrashItem {
  id: string;
  path: string;
  name: string;
  isDir: boolean;
  size: number;
  deletedAt: string;
  by: string;
}

interface TrashListing {
  enabled: boolean;
  maxBytes: number;
  days: number;
  totalBytes: number;
  items: TrashItem[];
}
