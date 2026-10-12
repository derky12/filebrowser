import { describe, expect, it } from "vitest";
import {
  changedRows,
  defaultOptions,
  hasErrors,
  planRenames,
  renameSteps,
  splitExtension,
} from "../batchRename";

const files = (...names: string[]) =>
  names.map((name) => ({ name, isDir: false }));
const opts = (o: Partial<ReturnType<typeof defaultOptions>>) => ({
  ...defaultOptions(),
  ...o,
});
const newNames = (rows: { newName: string }[]) => rows.map((r) => r.newName);

describe("splitExtension", () => {
  it("keeps the last extension and ignores folders and dotfiles", () => {
    expect(splitExtension("a.tar.gz", false)).toEqual(["a.tar", ".gz"]);
    expect(splitExtension("noext", false)).toEqual(["noext", ""]);
    expect(splitExtension(".bashrc", false)).toEqual([".bashrc", ""]);
    expect(splitExtension("v1.2", true)).toEqual(["v1.2", ""]);
  });
});

describe("planRenames", () => {
  it("replaces text in the base name only", () => {
    const rows = planRenames(
      files("IMG_001.jpg", "IMG_002.jpg"),
      opts({ find: "IMG", replace: "photo" })
    );
    expect(newNames(rows)).toEqual(["photo_001.jpg", "photo_002.jpg"]);
    // The extension is never touched.
    expect(
      newNames(
        planRenames(files("jpg.jpg"), opts({ find: "jpg", replace: "x" }))
      )
    ).toEqual(["x.jpg"]);
  });

  it("supports regular expressions with groups and case-insensitive matching", () => {
    const rows = planRenames(
      files("report-2024-01.txt"),
      opts({ find: "(\\d{4})-(\\d{2})", replace: "$2.$1", useRegex: true })
    );
    expect(newNames(rows)).toEqual(["report-01.2024.txt"]);
    expect(
      newNames(
        planRenames(
          files("ABC.txt"),
          opts({ find: "b", replace: "x", matchCase: false })
        )
      )
    ).toEqual(["AxC.txt"]);
    expect(
      newNames(
        planRenames(
          files("ABC.txt"),
          opts({ find: "b", replace: "x", matchCase: true })
        )
      )
    ).toEqual(["ABC.txt"]);
  });

  it("reports an invalid regular expression", () => {
    const rows = planRenames(
      files("a.txt"),
      opts({ find: "(", useRegex: true })
    );
    expect(rows[0].error).toBe("regex");
    expect(hasErrors(rows)).toBe(true);
  });

  it("treats special characters literally without the regex option", () => {
    expect(
      newNames(
        planRenames(files("a(1).txt"), opts({ find: "(1)", replace: "" }))
      )
    ).toEqual(["a.txt"]);
  });

  it("changes case, adds a prefix and a suffix", () => {
    expect(
      newNames(
        planRenames(files("hello world.txt"), opts({ caseMode: "upper" }))
      )
    ).toEqual(["HELLO WORLD.txt"]);
    expect(
      newNames(
        planRenames(files("HELLO WORLD.txt"), opts({ caseMode: "title" }))
      )
    ).toEqual(["Hello World.txt"]);
    expect(
      newNames(
        planRenames(files("a.txt"), opts({ prefix: "old_", suffix: "_v2" }))
      )
    ).toEqual(["old_a_v2.txt"]);
  });

  it("numbers the items with padding, start and step", () => {
    const rows = planRenames(
      files("a.jpg", "b.jpg", "c.jpg"),
      opts({ numbering: true, start: 8, step: 2, digits: 3, separator: "-" })
    );
    expect(newNames(rows)).toEqual(["a-008.jpg", "b-010.jpg", "c-012.jpg"]);
    expect(
      newNames(
        planRenames(
          files("a.jpg"),
          opts({ numbering: true, numberAtStart: true, digits: 1 })
        )
      )
    ).toEqual(["1_a.jpg"]);
  });

  it("marks what does not change", () => {
    const rows = planRenames(
      files("a.txt", "b.txt"),
      opts({ find: "a", replace: "z" })
    );
    expect(rows.map((r) => r.changed)).toEqual([true, false]);
    expect(changedRows(rows)).toHaveLength(1);
  });

  it("finds duplicates, existing names, empty and invalid names", () => {
    expect(
      planRenames(
        files("a.txt", "b.txt"),
        opts({ find: "[ab]", replace: "x", useRegex: true })
      ).map((r) => r.error)
    ).toEqual(["duplicate", "duplicate"]);
    expect(
      planRenames(
        files("a.txt"),
        opts({ find: "a", replace: "taken" }),
        new Set(["taken.txt"])
      )[0].error
    ).toBe("exists");
    expect(
      planRenames(files("a"), opts({ find: "a", replace: "" }))[0].error
    ).toBe("empty");
    expect(
      planRenames(files("a.txt"), opts({ find: "a", replace: "x/y" }))[0].error
    ).toBe("invalid");
    expect(
      planRenames(
        files("a.txt"),
        opts({ find: "a", replace: "é".repeat(200) })
      )[0].error
    ).toBe("tooLong");
  });
});

describe("renameSteps", () => {
  it("renames directly when no new name is still in use", () => {
    const steps = renameSteps(
      planRenames(files("a.txt", "b.txt"), opts({ prefix: "x_" }))
    );
    expect(steps).toEqual([
      { from: "a.txt", to: "x_a.txt" },
      { from: "b.txt", to: "x_b.txt" },
    ]);
  });

  it("goes through temporary names for a swap", () => {
    const rows = [
      { oldName: "a.txt", newName: "b.txt", changed: true },
      { oldName: "b.txt", newName: "a.txt", changed: true },
    ];
    const steps = renameSteps(rows);
    expect(steps).toHaveLength(4);
    // Nothing is renamed onto a name that is still in use.
    const names = new Set(["a.txt", "b.txt"]);
    for (const step of steps) {
      expect(names.has(step.to)).toBe(false);
      names.delete(step.from);
      names.add(step.to);
    }
    expect([...names].sort()).toEqual(["a.txt", "b.txt"]);
  });
});
