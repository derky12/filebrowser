// Transfer speeds are derived from the cumulative byte counters Syncthing
// reports per connection, by comparing two consecutive polls.

export interface TransferSample {
  time: number; // ms, server clock
  inBytes: number;
  outBytes: number;
}

export interface TransferRate {
  down: number; // bytes per second
  up: number;
}

// Returns null when there is nothing to compare yet. A counter that went
// backwards (the connection was re-established) counts as no transfer.
export function transferRate(
  previous: TransferSample | undefined,
  current: TransferSample
): TransferRate | null {
  if (!previous) return null;

  const seconds = (current.time - previous.time) / 1000;
  if (seconds <= 0) return null;

  return {
    down: Math.max(0, current.inBytes - previous.inBytes) / seconds,
    up: Math.max(0, current.outBytes - previous.outBytes) / seconds,
  };
}
