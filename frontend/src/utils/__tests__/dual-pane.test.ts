import { describe, expect, it } from "vitest";
import {
  clickSelection,
  crumbsOf,
  joinPath,
  parentOf,
  sortEntries,
} from "../dualPane";

const e = (
  name: string,
  isDir: boolean,
  size = 0,
  modified = "2024-01-01T00:00:00Z"
) => ({
  name,
  isDir,
  size,
  modified,
});

describe("sortEntries", () => {
  const list = [
    e("b.txt", false, 5),
    e("a.txt", false, 50),
    e("Zdir", true),
    e("adir", true),
    e("file10", false, 1),
    e("file2", false, 1),
  ];

  it("puts folders first and sorts names naturally, ignoring case", () => {
    expect(sortEntries(list, "name", true).map((x) => x.name)).toEqual([
      "adir",
      "Zdir",
      "a.txt",
      "b.txt",
      "file2",
      "file10",
    ]);
    expect(sortEntries(list, "name", false).map((x) => x.name)).toEqual([
      "Zdir",
      "adir",
      "file10",
      "file2",
      "b.txt",
      "a.txt",
    ]);
  });

  it("sorts by size and by date, keeping folders first", () => {
    expect(
      sortEntries(list, "size", false)
        .slice(2, 4)
        .map((x) => x.name)
    ).toEqual(["a.txt", "b.txt"]);
    const dated = [
      e("old", false, 1, "2020-01-01T00:00:00Z"),
      e("new", false, 1, "2024-01-01T00:00:00Z"),
    ];
    expect(sortEntries(dated, "modified", false).map((x) => x.name)).toEqual([
      "new",
      "old",
    ]);
  });

  it("does not change its input", () => {
    const copy = [...list];
    sortEntries(list, "size", true);
    expect(list).toEqual(copy);
  });
});

describe("paths", () => {
  it("builds crumbs, parents and joined paths", () => {
    expect(crumbsOf("/a/b")).toEqual([
      { name: "a", path: "/a" },
      { name: "b", path: "/a/b" },
    ]);
    expect(crumbsOf("/")).toEqual([]);
    expect(parentOf("/a/b")).toBe("/a");
    expect(parentOf("/a")).toBe("/");
    expect(parentOf("/")).toBe("/");
    expect(joinPath("/", "x")).toBe("/x");
    expect(joinPath("/a", "x")).toBe("/a/x");
  });
});

describe("clickSelection", () => {
  const names = ["a", "b", "c", "d", "e"];

  it("selects one item on a plain click", () => {
    const r = clickSelection(names, new Set(["a", "b"]), "a", "c", {
      toggle: false,
      range: false,
    });
    expect([...r.selected]).toEqual(["c"]);
    expect(r.anchor).toBe("c");
  });

  it("toggles with Ctrl/Cmd", () => {
    const r = clickSelection(names, new Set(["a"]), "a", "c", {
      toggle: true,
      range: false,
    });
    expect([...r.selected].sort()).toEqual(["a", "c"]);
    const off = clickSelection(names, r.selected, r.anchor, "a", {
      toggle: true,
      range: false,
    });
    expect([...off.selected]).toEqual(["c"]);
  });

  it("selects a range with Shift, in either direction, from the anchor", () => {
    const down = clickSelection(names, new Set(["b"]), "b", "d", {
      toggle: false,
      range: true,
    });
    expect([...down.selected]).toEqual(["b", "c", "d"]);
    expect(down.anchor).toBe("b");
    const up = clickSelection(names, new Set(["d"]), "d", "a", {
      toggle: false,
      range: true,
    });
    expect([...up.selected]).toEqual(["a", "b", "c", "d"]);
  });

  it("falls back to a single selection when there is no anchor", () => {
    const r = clickSelection(names, new Set(), null, "c", {
      toggle: false,
      range: true,
    });
    expect([...r.selected]).toEqual(["c"]);
  });
});
