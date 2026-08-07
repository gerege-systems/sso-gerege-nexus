/**
 * Per-language overlays layered over the mn/en dictionary.
 *
 * Lookup order in `t()` is overlay → entry[locale] → entry.en, so a term is
 * translated the moment it appears here and falls back to English until then.
 * That is what lets a locale be switched on while it is still only partly
 * translated, instead of waiting for all 552 keys.
 */
import type { Locale } from "../index";
import { ar } from "./ar";
import { zh } from "./zh";
import { fr } from "./fr";
import { ru } from "./ru";
import { es } from "./es";

export const overlays: Partial<Record<Locale, Record<string, string>>> = { ar, zh, fr, ru, es };
