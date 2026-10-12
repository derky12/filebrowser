// Formatting for the editor, chosen by file extension. Everything runs in the
// browser (Prettier, Ruff for Python, sql-formatter) except Go, which the
// server formats with gofmt. The heavy parts are imported lazily, so the
// editor only downloads what the open file needs.

export interface IndentOptions {
  tabWidth: number;
  useTabs: boolean;
}

export type FormatterKind = "prettier" | "ruff" | "sql" | "go";

export interface Formatter {
  kind: FormatterKind;
  label: string;
  parser?: string; // Prettier parser
}

const PRETTIER: Record<string, Formatter> = {
  yml: { kind: "prettier", label: "YAML", parser: "yaml" },
  yaml: { kind: "prettier", label: "YAML", parser: "yaml" },
  json: { kind: "prettier", label: "JSON", parser: "json" },
  jsonc: { kind: "prettier", label: "JSON", parser: "jsonc" },
  json5: { kind: "prettier", label: "JSON5", parser: "json5" },
  js: { kind: "prettier", label: "JavaScript", parser: "babel" },
  jsx: { kind: "prettier", label: "JavaScript", parser: "babel" },
  mjs: { kind: "prettier", label: "JavaScript", parser: "babel" },
  cjs: { kind: "prettier", label: "JavaScript", parser: "babel" },
  ts: { kind: "prettier", label: "TypeScript", parser: "typescript" },
  mts: { kind: "prettier", label: "TypeScript", parser: "typescript" },
  cts: { kind: "prettier", label: "TypeScript", parser: "typescript" },
  tsx: { kind: "prettier", label: "TypeScript", parser: "typescript" },
  css: { kind: "prettier", label: "CSS", parser: "css" },
  scss: { kind: "prettier", label: "SCSS", parser: "scss" },
  less: { kind: "prettier", label: "Less", parser: "less" },
  html: { kind: "prettier", label: "HTML", parser: "html" },
  htm: { kind: "prettier", label: "HTML", parser: "html" },
  vue: { kind: "prettier", label: "Vue", parser: "vue" },
  md: { kind: "prettier", label: "Markdown", parser: "markdown" },
  markdown: { kind: "prettier", label: "Markdown", parser: "markdown" },
};

const OTHERS: Record<string, Formatter> = {
  py: { kind: "ruff", label: "Python" },
  pyi: { kind: "ruff", label: "Python" },
  sql: { kind: "sql", label: "SQL" },
  go: { kind: "go", label: "Go" },
};

const extensionOf = (fileName: string) => {
  const name = fileName.split("/").pop() ?? fileName;
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
};

// Returns the formatter for a file name, or null when there is none.
export function formatterFor(fileName: string): Formatter | null {
  const ext = extensionOf(fileName);
  return PRETTIER[ext] ?? OTHERS[ext] ?? null;
}

// Indentation that matches the language's usual style; used both to configure
// the editor and as the formatter's indentation.
export function indentFor(fileName: string): IndentOptions {
  const name = (fileName.split("/").pop() ?? fileName).toLowerCase();
  const ext = extensionOf(fileName);

  if (ext === "go" || name === "makefile" || ext === "mk") {
    return { tabWidth: 4, useTabs: true };
  }
  if (
    ["py", "pyi", "rs", "java", "kt", "c", "h", "cpp", "hpp", "cs"].includes(
      ext
    )
  ) {
    return { tabWidth: 4, useTabs: false };
  }
  if (
    [
      "yml",
      "yaml",
      "json",
      "jsonc",
      "json5",
      "js",
      "jsx",
      "mjs",
      "cjs",
      "ts",
      "mts",
      "cts",
      "tsx",
      "css",
      "scss",
      "less",
      "html",
      "htm",
      "vue",
      "md",
      "markdown",
      "xml",
      "svg",
      "toml",
      "tf",
    ].includes(ext)
  ) {
    return { tabWidth: 2, useTabs: false };
  }

  return { tabWidth: 4, useTabs: false };
}

export interface FormatResult {
  text: string;
  cursor: number;
}

// Where the Ruff WebAssembly module is fetched from. Overridable for tests.
export const loaders = {
  ruffWasm: async (): Promise<unknown> =>
    (await import("@astral-sh/ruff-wasm-web/ruff_wasm_bg.wasm?url")).default,
};

let ruffReady: Promise<typeof import("@astral-sh/ruff-wasm-web")> | null = null;

const loadRuff = () => {
  if (!ruffReady) {
    ruffReady = (async () => {
      const ruff = await import("@astral-sh/ruff-wasm-web");
      await ruff.default({
        module_or_path: (await loaders.ruffWasm()) as never,
      });
      return ruff;
    })();
    // Allow another attempt after a failed download.
    ruffReady.catch(() => (ruffReady = null));
  }

  return ruffReady;
};

const prettierPlugins = async (parser: string) => {
  switch (parser) {
    case "yaml":
      return [import("prettier/plugins/yaml")];
    case "json":
    case "jsonc":
    case "json5":
    case "babel":
      return [
        import("prettier/plugins/babel"),
        import("prettier/plugins/estree"),
      ];
    case "typescript":
      return [
        import("prettier/plugins/typescript"),
        import("prettier/plugins/estree"),
      ];
    case "css":
    case "scss":
    case "less":
      return [import("prettier/plugins/postcss")];
    case "html":
    case "vue":
      return [
        import("prettier/plugins/html"),
        import("prettier/plugins/babel"),
        import("prettier/plugins/estree"),
        import("prettier/plugins/typescript"),
        import("prettier/plugins/postcss"),
      ];
    case "markdown":
      return [import("prettier/plugins/markdown")];
    default:
      return [];
  }
};

// Formats text for the given file name. Throws when there is no formatter or
// the text cannot be parsed; the text is never partially changed.
export async function formatCode(
  fileName: string,
  text: string,
  cursor: number,
  indent: IndentOptions = indentFor(fileName)
): Promise<FormatResult> {
  const formatter = formatterFor(fileName);
  if (!formatter) throw new Error(`No formatter for ${fileName}`);

  switch (formatter.kind) {
    case "prettier": {
      const { formatWithCursor } = await import("prettier/standalone");
      const plugins = await Promise.all(
        await prettierPlugins(formatter.parser!)
      );
      const out = await formatWithCursor(text, {
        parser: formatter.parser,
        plugins: plugins.map((p) => p.default ?? p) as never,
        tabWidth: indent.tabWidth,
        useTabs: indent.useTabs,
        printWidth: 100,
        cursorOffset: cursor,
      });
      return { text: out.formatted, cursor: Math.max(0, out.cursorOffset) };
    }
    case "ruff": {
      const ruff = await loadRuff();
      const workspace = new ruff.Workspace(
        {
          "line-length": 88,
          "indent-width": indent.tabWidth,
          format: { "indent-style": indent.useTabs ? "tab" : "space" },
        },
        ruff.PositionEncoding.Utf16
      );
      try {
        return {
          text: workspace.format(text),
          cursor: Math.min(cursor, text.length),
        };
      } finally {
        workspace.free();
      }
    }
    case "sql": {
      const { format } = await import("sql-formatter");
      return {
        text: format(text, {
          language: "sql",
          tabWidth: indent.tabWidth,
          useTabs: indent.useTabs,
          keywordCase: "upper",
        }),
        cursor: Math.min(cursor, text.length),
      };
    }
    case "go": {
      const { default: fetchURL } = await import("@/api/formatGo");
      return {
        text: await fetchURL(text),
        cursor: Math.min(cursor, text.length),
      };
    }
  }
}
