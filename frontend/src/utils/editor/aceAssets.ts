// Ace loads language modes, themes, snippets and syntax-check workers on
// demand. Its default is a CDN, which the app's content security policy blocks
// and which a private network cannot reach anyway. The aceAssets plugin in
// vite.config.ts ships the files with the app; this registers their URLs.
import ace from "ace-builds";
import { themesByName } from "ace-builds/src-noconflict/ext-themelist";
import { ACE_DIR, ACE_MODES, ACE_WORKERS } from "./aceFiles";

const url = (file: string) => {
  const prepend = (
    window as unknown as {
      __prependStaticUrl?: (path: string) => string;
    }
  ).__prependStaticUrl;
  const path = `${ACE_DIR}/${file}`;

  return prepend ? prepend(path) : `/${path}`;
};

let ready = false;

// Registers every bundled Ace asset. Safe to call more than once.
export function setupAceAssets() {
  if (ready) return;
  ready = true;

  for (const mode of ACE_MODES) {
    ace.config.setModuleUrl(`ace/mode/${mode}`, url(`mode-${mode}.js`));
    ace.config.setModuleUrl(`ace/snippets/${mode}`, url(`snippets/${mode}.js`));
  }
  for (const worker of ACE_WORKERS) {
    ace.config.setModuleUrl(
      `ace/mode/${worker}_worker`,
      url(`worker-${worker}.js`)
    );
  }
  for (const theme of Object.keys(themesByName)) {
    ace.config.setModuleUrl(`ace/theme/${theme}`, url(`theme-${theme}.js`));
  }
}
