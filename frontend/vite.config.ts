import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";
import fs from "node:fs";
import { ACE_DIR, ACE_MODES, ACE_WORKERS } from "./src/utils/editor/aceFiles";

// Copies the Ace language modes, themes, snippets and workers the editor uses
// next to the other assets. They are plain scripts that Ace loads by URL, so
// they are copied as they are instead of being processed as modules.
const aceAssets = () => ({
  name: "ace-assets",
  apply: "build" as const,
  generateBundle(this: { emitFile: (file: object) => void }) {
    const dir = path.resolve(
      __dirname,
      "node_modules/ace-builds/src-min-noconflict"
    );
    const emit = (file: string) => {
      const source = path.join(dir, file);
      if (!fs.existsSync(source)) return;
      this.emitFile({
        type: "asset",
        fileName: `${ACE_DIR}/${file}`,
        source: fs.readFileSync(source),
      });
    };

    for (const mode of ACE_MODES) {
      emit(`mode-${mode}.js`);
      emit(`snippets/${mode}.js`);
    }
    for (const worker of ACE_WORKERS) emit(`worker-${worker}.js`);
    for (const file of fs.readdirSync(dir)) {
      if (/^theme-.*\.js$/.test(file)) emit(file);
    }
  },
});

const plugins = [
  vue(),
  VueI18nPlugin({
    include: [path.resolve(__dirname, "./src/i18n/**/*.json")],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  aceAssets(),
  compression({ include: /\.js$/, deleteOriginalAssets: false }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(__dirname, "src")}/`,
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: "ws://127.0.0.1:8080",
            ws: true,
          },
          "/api": "http://127.0.0.1:8080",
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(__dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
