import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";
import {
  formatCode,
  formatterFor,
  indentFor,
  loaders,
} from "../editor/formatters";

// The Go formatter calls the server; everything else runs locally.
vi.mock("@/api/formatGo", () => ({
  default: vi.fn(async (source: string) => `formatted:${source}`),
}));

loaders.ruffWasm = async () =>
  readFileSync(
    fileURLToPath(
      new URL(
        "../../../node_modules/@astral-sh/ruff-wasm-web/ruff_wasm_bg.wasm",
        import.meta.url
      )
    )
  );

describe("formatterFor", () => {
  it("picks a formatter by extension", () => {
    expect(formatterFor("docker-compose.yml")?.kind).toBe("prettier");
    expect(formatterFor("a/b/app.py")?.kind).toBe("ruff");
    expect(formatterFor("main.go")?.kind).toBe("go");
    expect(formatterFor("query.SQL")?.kind).toBe("sql");
    expect(formatterFor("types.d.ts")?.parser).toBe("typescript");
    expect(formatterFor("README.md")?.parser).toBe("markdown");
  });

  it("has none for unknown or extension-less files", () => {
    expect(formatterFor("notes.txt")).toBeNull();
    expect(formatterFor("Makefile")).toBeNull();
    expect(formatterFor(".bashrc")).toBeNull();
  });
});

describe("indentFor", () => {
  it("follows each language's usual style", () => {
    expect(indentFor("a.yml")).toEqual({ tabWidth: 2, useTabs: false });
    expect(indentFor("a.py")).toEqual({ tabWidth: 4, useTabs: false });
    expect(indentFor("a.go")).toEqual({ tabWidth: 4, useTabs: true });
    expect(indentFor("Makefile")).toEqual({ tabWidth: 4, useTabs: true });
    expect(indentFor("a.unknown")).toEqual({ tabWidth: 4, useTabs: false });
  });
});

describe("formatCode", () => {
  it("formats YAML", async () => {
    const out = await formatCode(
      "a.yml",
      "a:   1\nb:\n      - x\n      -   y\n",
      0
    );
    expect(out.text).toBe("a: 1\nb:\n  - x\n  - y\n");
  });

  it("formats JSON", async () => {
    const out = await formatCode("a.json", '{"a":1,"b":[1,2]}', 0);
    expect(out.text).toBe('{ "a": 1, "b": [1, 2] }\n');
  });

  it("formats JavaScript and keeps the cursor on the same line", async () => {
    const src = "const  a  =  1;\nfunction f( x ){return x+a}\n";
    // Cursor inside the word "return".
    const out = await formatCode("a.js", src, src.indexOf("return") + 3);
    expect(out.text).toBe(
      "const a = 1;\nfunction f(x) {\n  return x + a;\n}\n"
    );
    const line = out.text.slice(0, out.cursor).split("\n").length - 1;
    expect(out.text.split("\n")[line]).toContain("return");
  });

  it("formats TypeScript", async () => {
    const out = await formatCode("a.ts", "let x:number=1\n", 0);
    expect(out.text).toBe("let x: number = 1;\n");
  });

  it("formats CSS", async () => {
    const out = await formatCode("a.css", "a{color:red;margin:0}", 0);
    expect(out.text).toBe("a {\n  color: red;\n  margin: 0;\n}\n");
  });

  it("formats HTML", async () => {
    const out = await formatCode("a.html", "<div><p>hi</p></div>", 0);
    expect(out.text).toContain("<p>hi</p>");
    expect(out.text.endsWith("\n")).toBe(true);
  });

  it("formats Markdown", async () => {
    const out = await formatCode("a.md", "#  Title\n\n*  one\n*  two\n", 0);
    expect(out.text).toBe("# Title\n\n- one\n- two\n");
  });

  it("formats SQL", async () => {
    const out = await formatCode("a.sql", "select a,b from t where a=1", 0);
    expect(out.text).toContain("SELECT");
    expect(out.text).toContain("\n");
  });

  it("formats Python with Ruff", async () => {
    const out = await formatCode(
      "a.py",
      "def f( a,b ):\n  return a+b\nprint( 'x' )\n",
      0
    );
    expect(out.text).toBe('def f(a, b):\n    return a + b\n\n\nprint("x")\n');
  });

  it("formats Go through the server", async () => {
    const out = await formatCode("a.go", "package main", 3);
    expect(out.text).toBe("formatted:package main");
  });

  it("reports syntax errors instead of changing anything", async () => {
    await expect(formatCode("a.json", '{"a":', 0)).rejects.toThrow();
    await expect(formatCode("a.py", "def f(:\n", 0)).rejects.toThrow();
    await expect(formatCode("a.yml", "a: [1, 2\n", 0)).rejects.toThrow();
  });

  it("refuses files without a formatter", async () => {
    await expect(formatCode("a.txt", "x", 0)).rejects.toThrow("No formatter");
  });
});
