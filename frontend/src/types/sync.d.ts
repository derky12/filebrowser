type SyncMode = "oneway" | "twoway" | "receive";

interface SyncDevice {
  id: string;
  name: string;
  address: string;
  connected: boolean;
  paused: boolean;
}

interface SyncRemote {
  deviceID: string;
  connected: boolean;
  completion: number;
}

interface SyncFolder {
  id: string;
  label: string;
  path: string;
  external: boolean;
  mode: SyncMode;
  paused: boolean;
  state: string;
  globalBytes: number;
  inSyncBytes: number;
  needBytes: number;
  error?: string;
  remotes: SyncRemote[];
}

interface SyncPendingDevice {
  id: string;
  name: string;
  address: string;
  time: string;
}

interface SyncPendingFolder {
  id: string;
  label: string;
  deviceID: string;
  deviceName: string;
  time: string;
}

interface SyncStatus {
  deviceID: string;
  devices: SyncDevice[];
  folders: SyncFolder[];
  pendingDevices: SyncPendingDevice[];
  pendingFolders: SyncPendingFolder[];
}
