interface PermissionsInfo {
  path: string;
  isDir: boolean;
  mode: string; // octal, e.g. "0644"
  uid: number;
  gid: number;
  owner: string;
  group: string;
  canChown: boolean;
}

interface PermissionsChange {
  mode?: string;
  recursive?: boolean;
  fileMode?: string;
  dirMode?: string;
  owner?: string;
  group?: string;
}

interface PermissionsResult {
  changed: number;
  skipped: number;
  failed: number;
  errors: string[];
}
