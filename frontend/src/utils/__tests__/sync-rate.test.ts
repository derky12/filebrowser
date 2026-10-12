import { describe, expect, it } from "vitest";
import { transferRate } from "../syncRate";

describe("transferRate", () => {
  it("needs a previous sample", () => {
    expect(
      transferRate(undefined, { time: 1000, inBytes: 5, outBytes: 5 })
    ).toBeNull();
  });

  it("divides the byte difference by the elapsed seconds", () => {
    const rate = transferRate(
      { time: 0, inBytes: 1000, outBytes: 0 },
      { time: 4000, inBytes: 5000, outBytes: 2000 }
    );
    expect(rate).toEqual({ down: 1000, up: 500 });
  });

  it("treats a reset counter as no transfer", () => {
    const rate = transferRate(
      { time: 0, inBytes: 9000, outBytes: 9000 },
      { time: 2000, inBytes: 100, outBytes: 100 }
    );
    expect(rate).toEqual({ down: 0, up: 0 });
  });

  it("ignores samples that are not newer", () => {
    expect(
      transferRate(
        { time: 5000, inBytes: 0, outBytes: 0 },
        { time: 5000, inBytes: 10, outBytes: 10 }
      )
    ).toBeNull();
  });
});
