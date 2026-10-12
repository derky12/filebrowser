// Renaming several items at once: the rules are applied to the base name (the
// extension of files is kept), the result is checked for problems before
// anything is renamed.

export interface RenameInput {
  name: string;
  isDir: boolean;
}

export interface BatchOptions {
  find: string;
  replace: string;
  useRegex: boolean;
  matchCase: boolean;
  caseMode: "none" | "lower" | "upper" | "title";
  prefix: string;
  suffix: string;
  numbering: boolean;
  start: number;
  step: number;
  digits: number; // zero padding
  separator: string;
  numberAtStart: boolean;
}

export const defaultOptions = (): BatchOptions => ({
  find: "",
  replace: "",
  useRegex: false,
  matchCase: true,
  caseMode: "none",
  prefix: "",
  suffix: "",
  numbering: false,
  start: 1,
  step: 1,
  digits: 2,
  separator: "_",
  numberAtStart: false,
});

export interface RenameRow {
  oldName: string;
  newName: string;
  changed: boolean;
  error?: "empty" | "invalid" | "tooLong" | "duplicate" | "exists" | "regex";
}

// Splits "a.tar.gz" into ["a.tar", ".gz"]; folders and dotfiles have no extension.
export const splitExtension = (
  name: string,
  isDir: boolean
): [string, string] => {
  if (isDir) return [name, ""];
  const dot = name.lastIndexOf(".");
  return dot > 0 ? [name.slice(0, dot), name.slice(dot)] : [name, ""];
};

const titleCase = (text: string) =>
  text.replace(
    /\p{L}[\p{L}\p{N}']*/gu,
    (word) => word[0].toUpperCase() + word.slice(1).toLowerCase()
  );

const escapeRegExp = (text: string) =>
  text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// Builds the matcher of the find field, or null when it is empty or invalid.
const matcher = (o: BatchOptions): RegExp | null | "invalid" => {
  if (o.find === "") return null;

  try {
    return new RegExp(
      o.useRegex ? o.find : escapeRegExp(o.find),
      o.matchCase ? "g" : "gi"
    );
  } catch {
    return "invalid";
  }
};

const pad = (n: number, digits: number) =>
  String(n).padStart(Math.max(1, digits), "0");

export function planRenames(
  items: RenameInput[],
  options: BatchOptions,
  existing: ReadonlySet<string> = new Set()
): RenameRow[] {
  const re = matcher(options);
  const rows: RenameRow[] = items.map((item, index) => {
    if (re === "invalid") {
      return {
        oldName: item.name,
        newName: item.name,
        changed: false,
        error: "regex" as const,
      };
    }

    const [base, ext] = splitExtension(item.name, item.isDir);
    let next = re ? base.replace(re, options.replace) : base;

    if (options.caseMode === "lower") next = next.toLowerCase();
    else if (options.caseMode === "upper") next = next.toUpperCase();
    else if (options.caseMode === "title") next = titleCase(next);

    next = options.prefix + next + options.suffix;

    if (options.numbering) {
      const n = pad(options.start + index * options.step, options.digits);
      next = options.numberAtStart
        ? n + options.separator + next
        : next + options.separator + n;
    }

    const newName = next + ext;
    return { oldName: item.name, newName, changed: newName !== item.name };
  });

  const selected = new Set(items.map((i) => i.name));
  const seen = new Map<string, number>();
  for (const row of rows)
    seen.set(row.newName, (seen.get(row.newName) ?? 0) + 1);

  for (const row of rows) {
    if (row.error) continue;
    if (row.newName.trim() === "") row.error = "empty";
    else if (
      /[/\0]/.test(row.newName) ||
      row.newName === "." ||
      row.newName === ".."
    )
      row.error = "invalid";
    else if (new TextEncoder().encode(row.newName).length > 255)
      row.error = "tooLong";
    else if ((seen.get(row.newName) ?? 0) > 1) row.error = "duplicate";
    else if (
      row.changed &&
      existing.has(row.newName) &&
      !selected.has(row.newName)
    )
      row.error = "exists";
  }

  return rows;
}

export const hasErrors = (rows: RenameRow[]) => rows.some((r) => r.error);
export const changedRows = (rows: RenameRow[]) => rows.filter((r) => r.changed);

// One step of a rename: from -> to. Names are renamed through temporary names
// first when a new name is the current name of another renamed item (a swap or
// a shift by one), so that no step lands on a name still in use.
export function renameSteps(rows: RenameRow[]): { from: string; to: string }[] {
  const changed = changedRows(rows);
  const oldNames = new Set(changed.map((r) => r.oldName));
  const needsTemp = changed.some((r) => oldNames.has(r.newName));
  if (!needsTemp)
    return changed.map((r) => ({ from: r.oldName, to: r.newName }));

  const stamp = Date.now().toString(36);
  const temp = (i: number) => `.rename-${stamp}-${i}`;
  return [
    ...changed.map((r, i) => ({ from: r.oldName, to: temp(i) })),
    ...changed.map((r, i) => ({ from: temp(i), to: r.newName })),
  ];
}
