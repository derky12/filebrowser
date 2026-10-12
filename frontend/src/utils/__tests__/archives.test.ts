import { describe, expect, it } from "vitest";
import { archiveBaseName, isArchiveName } from "../archives";

describe("archives", () => {
  it("recognizes extractable names, case-insensitively", () => {
    for (const name of [
      "a.zip",
      "A.ZIP",
      "b.tar.gz",
      "c.tgz",
      "d.7z",
      "e.rar",
      "f.tar.zst",
    ]) {
      expect(isArchiveName(name)).toBe(true);
    }
    for (const name of ["a.txt", "zip", "a.gz", "archive.zip.txt"]) {
      expect(isArchiveName(name)).toBe(false);
    }
  });

  it("derives the folder name by removing the longest extension", () => {
    expect(archiveBaseName("photos.tar.gz")).toBe("photos");
    expect(archiveBaseName("My Docs.zip")).toBe("My Docs");
    expect(archiveBaseName("backup.2024.tar.xz")).toBe("backup.2024");
    expect(archiveBaseName("plain.txt")).toBe("plain.txt");
  });
});
