import { describe, expect, it } from "vitest";
import { chooseFolder, splitTyped, suggestFolders } from "../pathSuggest";

describe("splitTyped", () => {
  it("separates the folder to list from the name being typed", () => {
    expect(splitTyped("/a/b/c")).toEqual({ parent: "/a/b", prefix: "c" });
    expect(splitTyped("/a/b/")).toEqual({ parent: "/a/b", prefix: "" });
    expect(splitTyped("/a")).toEqual({ parent: "/", prefix: "a" });
    expect(splitTyped("/")).toEqual({ parent: "/", prefix: "" });
  });

  it("takes a path without a leading slash from the root", () => {
    expect(splitTyped("docs/ph")).toEqual({ parent: "/docs", prefix: "ph" });
    expect(splitTyped("")).toEqual({ parent: "/", prefix: "" });
  });

  it("copes with repeated slashes", () => {
    expect(splitTyped("/a//b")).toEqual({ parent: "/a", prefix: "b" });
  });
});

describe("suggestFolders", () => {
  const entries = [
    { name: "Photos", isDir: true },
    { name: "photo-old", isDir: true },
    { name: "photo.txt", isDir: false },
    { name: "docs", isDir: true },
    { name: ".hidden", isDir: true },
    { name: "p10", isDir: true },
    { name: "p2", isDir: true },
  ];

  it("offers folders only, matching the start case-insensitively", () => {
    expect(suggestFolders(entries, "pho")).toEqual(["photo-old", "Photos"]);
    expect(suggestFolders(entries, "PHOTOS")).toEqual(["Photos"]);
  });

  it("offers every visible folder for an empty prefix, in natural order", () => {
    expect(suggestFolders(entries, "")).toEqual([
      "docs",
      "p2",
      "p10",
      "photo-old",
      "Photos",
    ]);
  });

  it("hides dotted folders unless asked with a dot", () => {
    expect(suggestFolders(entries, "")).not.toContain(".hidden");
    expect(suggestFolders(entries, ".")).toEqual([".hidden"]);
  });

  it("limits the number of suggestions", () => {
    const many = Array.from({ length: 30 }, (_, i) => ({
      name: `d${i}`,
      isDir: true,
    }));
    expect(suggestFolders(many, "d", 5)).toHaveLength(5);
  });
});

describe("chooseFolder", () => {
  it("ends with a slash for the next level", () => {
    expect(chooseFolder("/", "docs")).toBe("/docs/");
    expect(chooseFolder("/a/b", "c")).toBe("/a/b/c/");
  });
});
