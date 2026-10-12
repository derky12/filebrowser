interface SyncConflict {
  folderID: string;
  folderLabel: string;
  path: string;
  original: string;
  size: number;
  modified: string;
  conflictedAt: string;
  originalExists: boolean;
  originalSize: number;
  originalModified: string;
}

interface SyncLimits {
  maxRecvKbps: number;
  maxSendKbps: number;
  limitBandwidthInLan: boolean;
}
