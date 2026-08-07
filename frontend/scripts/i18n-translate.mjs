#!/usr/bin/env node
/**
 * Fills a language overlay from the mn/en dictionary using Gemini.
 *
 *   npm run i18n:translate -- --locale fr            # show what would change
 *   npm run i18n:translate -- --locale fr --write    # write lib/i18n/locales/fr.ts
 *
 * The platform already talks to Gemini for the copilot and the translate panel;
 * this is the same model applied to the interface itself, at build time rather
 * than per request. Translating at request time would mean paying latency and
 * money on every screen and getting slightly different wording each visit —
 * for interface text, which changes rarely and must be stable, generating once
 * and committing the result is the cheaper and more honest trade.
 *
 * Three rules the generator holds itself to:
 *
 *   1. It never overwrites. Only keys missing from the overlay are sent, so a
 *      human correction is permanent and re-running is always safe.
 *   2. It never invents placeholders. `{name}` and friends are checked in the
 *      output and a translation that lost or renamed one is dropped, because a
 *      placeholder that no longer matches renders as literal braces to a user.
 *   3. It leaves technical identifiers alone — tenant, RBAC, OAuth2, eID, SKU,
 *      e-Barimt and the like are product vocabulary, not prose.
 *
 * What it produces is a draft. Machine translation of interface text is good
 * enough to review and wrong often enough that shipping it unread is not
 * acceptable — the header written into each overlay says so too.
 */

import { readFile, writeFile, readdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const I18N = path.join(HERE, "..", "lib", "i18n");
const OVERLAY_DIR = path.join(I18N, "locales");

const LANGUAGE_NAMES = {
  ar: "Arabic", zh: "Simplified Chinese", fr: "French", ru: "Russian", es: "Spanish",
};

/** Terms that must survive translation untouched. */
const KEEP = [
  "Gerege Nexus", "eID", "E-ID", "DAN", "XYP", "OAuth2", "OIDC", "SSO", "RBAC",
  "tenant", "SKU", "e-Barimt", "PIN2", "QR", "API", "PDF", "Gemini",
];

function arg(name, fallback = null) {
  const i = process.argv.indexOf(`--${name}`);
  if (i !== -1 && process.argv[i + 1] && !process.argv[i + 1].startsWith("--")) return process.argv[i + 1];
  return process.argv.includes(`--${name}`) ? true : fallback;
}

/**
 * Reads a dictionary module without a TypeScript loader.
 *
 * These files are `export const x = { ... }` where every value is a string
 * literal — data, not code. Stripping the export and evaluating the literal is
 * enough, and avoids taking a parser dependency to read files this repo owns.
 */
async function loadModule(file) {
  const src = await readFile(file, "utf8");
  const start = src.indexOf("{", src.indexOf("export const"));
  const end = src.lastIndexOf("}");
  if (start === -1 || end === -1) throw new Error(`cannot read object literal from ${file}`);
  const literal = src.slice(start, end + 1);
  return Function(`"use strict"; return (${literal});`)();
}

async function loadDictionary() {
  const files = [path.join(I18N, "base.ts"), path.join(I18N, "web.ts")];
  for (const f of (await readdir(path.join(I18N, "addons"))).sort()) {
    if (f.endsWith(".ts")) files.push(path.join(I18N, "addons", f));
  }
  const out = {};
  for (const f of files) Object.assign(out, await loadModule(f));
  return out;
}

const placeholders = (s) => (s.match(/\{[a-z_]+\}/gi) || []).sort().join(",");

async function callGemini(batch, locale) {
  const key = process.env.GEMINI_API_KEY;
  if (!key) throw new Error("GEMINI_API_KEY is not set");
  const model = process.env.GEMINI_MODEL || "gemini-2.5-flash";
  const base = process.env.GEMINI_API_BASE || "https://generativelanguage.googleapis.com";

  const prompt = [
    `Translate these user-interface strings into ${LANGUAGE_NAMES[locale]} (code ${locale}).`,
    `They come from Gerege Nexus, a platform connecting services, operations, systems and data`,
    `for public and private organizations. The tone is plain and professional.`,
    ``,
    `Rules:`,
    `- Return ONLY a JSON object mapping each id to its translation. No prose, no code fence.`,
    `- Keep placeholders such as {name} or {count} exactly as they appear, untranslated.`,
    `- Do not translate these terms: ${KEEP.join(", ")}.`,
    `- Keep it short. These are buttons, labels and menu entries, not documentation.`,
    `- The "mn" field is the Mongolian source and "en" the English rendering; prefer the`,
    `  Mongolian for meaning where the two differ.`,
    ``,
    JSON.stringify(batch, null, 1),
  ].join("\n");

  const res = await fetch(`${base}/v1beta/models/${model}:generateContent?key=${key}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      contents: [{ role: "user", parts: [{ text: prompt }] }],
      generationConfig: { temperature: 0.2, responseMimeType: "application/json" },
    }),
  });
  if (!res.ok) throw new Error(`Gemini ${res.status}: ${(await res.text()).slice(0, 300)}`);
  const body = await res.json();
  const text = body?.candidates?.[0]?.content?.parts?.map((p) => p.text).join("") ?? "";
  try {
    return JSON.parse(text);
  } catch {
    throw new Error(`Gemini did not return JSON: ${text.slice(0, 300)}`);
  }
}

async function main() {
  const locale = arg("locale");
  const write = arg("write") === true;
  const limit = Number(arg("limit", "0")) || 0;

  if (!locale || !LANGUAGE_NAMES[locale]) {
    console.error(`Usage: npm run i18n:translate -- --locale <${Object.keys(LANGUAGE_NAMES).join("|")}> [--write] [--limit N]`);
    process.exit(2);
  }

  const dictionary = await loadDictionary();
  const overlayFile = path.join(OVERLAY_DIR, `${locale}.ts`);
  const existing = existsSync(overlayFile) ? await loadModule(overlayFile) : {};

  let pending = Object.entries(dictionary).filter(([key]) => !existing[key]);
  if (limit) pending = pending.slice(0, limit);

  console.log(`${locale}: ${Object.keys(dictionary).length} keys, ${Object.keys(existing).length} already translated, ${pending.length} to do`);
  if (!pending.length) return;

  const translated = {};
  let rejected = 0;
  const SIZE = 40;
  for (let i = 0; i < pending.length; i += SIZE) {
    const slice = pending.slice(i, i + SIZE);
    const batch = Object.fromEntries(slice.map(([k, v]) => [k, { mn: v.mn, en: v.en }]));
    process.stdout.write(`  batch ${Math.floor(i / SIZE) + 1}/${Math.ceil(pending.length / SIZE)}… `);
    const answer = await callGemini(batch, locale);
    let ok = 0;
    for (const [key, value] of Object.entries(answer)) {
      const source = dictionary[key];
      if (!source || typeof value !== "string" || !value.trim()) continue;
      // A translation that dropped or renamed a placeholder would render the
      // braces to a user, so it is discarded rather than committed.
      if (placeholders(value) !== placeholders(source.en)) { rejected++; continue; }
      translated[key] = value.trim();
      ok++;
    }
    console.log(`${ok} ok`);
  }

  const merged = { ...existing, ...translated };
  const keys = Object.keys(merged).sort();
  const body = keys.map((k) => `  ${JSON.stringify(k)}: ${JSON.stringify(merged[k])},`).join("\n");
  const header = await readFile(overlayFile, "utf8").then((s) => s.slice(0, s.indexOf("export const")));
  const out = `${header}export const ${locale}: Record<string, string> = {\n${body}\n};\n`;

  console.log(`\n${Object.keys(translated).length} new, ${rejected} rejected for placeholder drift, ${keys.length} total`);
  if (!write) {
    console.log(`\nDry run. Re-run with --write to update ${path.relative(process.cwd(), overlayFile)}`);
    return;
  }
  await writeFile(overlayFile, out, "utf8");
  console.log(`Wrote ${path.relative(process.cwd(), overlayFile)} — review it before committing.`);
}

main().catch((err) => {
  console.error(String(err.message || err));
  process.exit(1);
});
