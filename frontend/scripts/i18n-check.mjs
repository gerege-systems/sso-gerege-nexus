#!/usr/bin/env node
/**
 * Audits the dictionary against the language overlays.
 *
 *   npm run i18n:check
 *
 * Two failures this catches, both of which are silent at runtime:
 *
 *   1. An **orphaned** overlay key — a translation whose dictionary key was
 *      renamed or deleted. Nothing reads it, so the screen it was written for
 *      quietly renders English forever.
 *   2. An **untranslated** key — one the dictionary has and the overlay does
 *      not. `t()` falls back to English by design, which is the right runtime
 *      behaviour and the wrong thing to discover from a screenshot. A screen
 *      showing Mongolian menus, Chinese headings and English buttons at the
 *      same time is what a pile of these looks like from the outside.
 *
 * Exits non-zero on either, so CI can hold the line the generator draws.
 */

import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const I18N = path.join(HERE, "..", "lib", "i18n");
const OVERLAYS = ["ar", "zh", "fr", "ru", "es"];

/**
 * Reads a dictionary module without a TypeScript loader — the same trick
 * scripts/i18n-translate.mjs uses. These files are `export const x = {...}`
 * where every value is a literal: data, not code.
 */
async function loadModule(file) {
  const src = await readFile(file, "utf8");
  const start = src.indexOf("{", src.indexOf("export const"));
  const end = src.lastIndexOf("}");
  if (start === -1 || end === -1) throw new Error(`cannot read object literal from ${file}`);
  return Function(`"use strict"; return (${src.slice(start, end + 1)});`)();
}

const files = [path.join(I18N, "base.ts"), path.join(I18N, "web.ts")];
for (const f of (await readdir(path.join(I18N, "addons"))).sort()) {
  if (f.endsWith(".ts")) files.push(path.join(I18N, "addons", f));
}

const dictionary = {};
for (const f of files) Object.assign(dictionary, await loadModule(f));
const keys = Object.keys(dictionary);

// mn and en are hand-authored on the entry itself, so they are checked here
// rather than in an overlay.
let failed = false;
for (const [key, entry] of Object.entries(dictionary)) {
  for (const required of ["mn", "en"]) {
    if (!entry?.[required]) {
      console.error(`✗ ${key}: missing ${required} in the source dictionary`);
      failed = true;
    }
  }
}

for (const locale of OVERLAYS) {
  const overlay = await loadModule(path.join(I18N, "locales", `${locale}.ts`));
  const orphaned = Object.keys(overlay).filter((k) => !(k in dictionary));
  const untranslated = keys.filter((k) => !overlay[k]);

  if (orphaned.length || untranslated.length) failed = true;
  const mark = orphaned.length || untranslated.length ? "✗" : "✓";
  console.log(
    `${mark} ${locale}: ${keys.length - untranslated.length}/${keys.length} translated` +
      (orphaned.length ? `, ${orphaned.length} orphaned` : ""),
  );
  for (const k of orphaned) console.error(`    orphaned: ${k}`);
  for (const k of untranslated) console.error(`    untranslated: ${k}`);
}

if (failed) {
  console.error("\nRun `npm run i18n:translate -- --locale <code> --write` to fill the gaps.");
  process.exit(1);
}
console.log(`\n${keys.length} keys, ${OVERLAYS.length} overlays, no gaps.`);
