"use client";

import React, { createContext, useContext, useEffect, useMemo, useState } from "react";

import { base } from "./base";
import { web } from "./web";
import { access } from "./addons/access";
import { ai } from "./addons/ai";
import { app_store } from "./addons/app_store";
import { appearance } from "./addons/appearance";
import { auth } from "./addons/auth";
import { billing } from "./addons/billing";
import { contacts } from "./addons/contacts";
import { developer } from "./addons/developer";
import { modules } from "./addons/modules";
import { documents } from "./addons/documents";
import { esign } from "./addons/esign";
import { gov } from "./addons/gov";
import { integrations } from "./addons/integrations";
import { inventory } from "./addons/inventory";
import { products } from "./addons/products";
import { website } from "./addons/website";
import { overlays } from "./locales";

export type Locale = "mn" | "ar" | "zh" | "en" | "fr" | "ru" | "es";

/**
 * Mongolian plus the six official languages of the United Nations, in the UN's
 * own alphabetical order. The list is deliberately a policy rather than a
 * wishlist: every entry here is one more column every future translation has to
 * fill, so growing it is a decision, not a convenience.
 */
export const LOCALES: { code: Locale; label: string; flag: string; rtl?: boolean }[] = [
  { code: "mn", label: "Монгол", flag: "/icons/flag-mn.png" },
  { code: "ar", label: "العربية", flag: "/icons/flag-ar.png", rtl: true },
  { code: "zh", label: "中文", flag: "/icons/flag-zh.png" },
  { code: "en", label: "English", flag: "/icons/flag-en.png" },
  { code: "fr", label: "Français", flag: "/icons/flag-fr.png" },
  { code: "ru", label: "Русский", flag: "/icons/flag-ru.png" },
  { code: "es", label: "Español", flag: "/icons/flag-es.png" },
];

/**
 * What a fresh install offers: Mongolian, the source language, and English, the
 * one every translation falls back to. These two cannot be switched off — that
 * is what keeps "turn everything off" from being a reachable state, so no guard
 * against an empty language list is needed anywhere else.
 *
 * The remaining five are opt-in per device from Settings → Appearance. They are
 * shipped but not offered, because the dictionary is not fully translated into
 * them yet: an untranslated term falls back to English (see `t` below), and
 * that is a reasonable screen to hand someone who asked for French, but not one
 * to hand everybody by default.
 */
export const DEFAULT_LOCALES: Locale[] = ["mn", "en"];
export const OPTIONAL_LOCALES: Locale[] = LOCALES.map((l) => l.code).filter(
  (code) => !DEFAULT_LOCALES.includes(code),
);

const STORAGE_KEY = "locale";
const ENABLED_STORAGE_KEY = "locales.enabled";
export const DEFAULT_LOCALE: Locale = "mn";

/**
 * The dictionary, assembled from one file per module the way Odoo gives each
 * addon its own translations. `base` and `web` are the shared ones: a term
 * that more than one screen shows belongs there, never duplicated per screen.
 *
 * Keys read `<module>.<kind>.<term>`, where kind classifies the term the way
 * Odoo does — field (a data label), action (a button), menu, state (a
 * selection value), model (a business object), view (screen copy) or message
 * (something said to the user).
 */
const dictionary = {
  ...base,
  ...web,
  ...access,
  ...ai,
  ...app_store,
  ...appearance,
  ...auth,
  ...billing,
  ...contacts,
  ...developer,
  ...modules,
  ...documents,
  ...esign,
  ...gov,
  ...integrations,
  ...inventory,
  ...products,
  ...website,
} as const;

export type TranslationKey = keyof typeof dictionary;

interface I18nValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  /** Locales offered in the switchers: the two defaults plus whatever is on. */
  availableLocales: Locale[];
  /** Turn one of the optional locales on or off. The defaults ignore this. */
  setLocaleEnabled: (locale: Locale, enabled: boolean) => void;
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | null>(null);

const ALL_CODES = LOCALES.map((entry) => entry.code);

function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (ALL_CODES as string[]).includes(value);
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  // Server and first client render must agree, so the stored preference is
  // applied in an effect rather than during initial state.
  const [locale, setLocaleState] = useState<Locale>(DEFAULT_LOCALE);
  const [extraLocales, setExtraLocales] = useState<Locale[]>([]);

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    let enabled: Locale[] = [];
    try {
      const raw = window.localStorage.getItem(ENABLED_STORAGE_KEY);
      const parsed: unknown = raw ? JSON.parse(raw) : [];
      if (Array.isArray(parsed)) {
        enabled = parsed.filter(isLocale).filter((code) => OPTIONAL_LOCALES.includes(code));
      }
    } catch {
      // A hand-edited or half-written value should cost the user their language
      // list, not the whole shell — fall back to shipping defaults.
      enabled = [];
    }
    setExtraLocales(enabled);
    // Only restore a stored language that is still on offer. Someone who picked
    // French and later switched it off would otherwise be stuck in a language
    // the switcher no longer shows.
    if (isLocale(stored) && (DEFAULT_LOCALES.includes(stored) || enabled.includes(stored))) {
      setLocaleState(stored);
    }
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
    // Arabic reverses the whole layout, so the document direction follows the
    // language rather than being set per component.
    document.documentElement.dir = LOCALES.find((l) => l.code === locale)?.rtl ? "rtl" : "ltr";
  }, [locale]);

  const value = useMemo<I18nValue>(() => {
    const availableLocales = ALL_CODES.filter(
      (code) => DEFAULT_LOCALES.includes(code) || extraLocales.includes(code),
    );

    const setLocale = (next: Locale) => {
      window.localStorage.setItem(STORAGE_KEY, next);
      setLocaleState(next);
    };

    const setLocaleEnabled = (target: Locale, enabled: boolean) => {
      if (DEFAULT_LOCALES.includes(target)) return; // mn/en are not switchable
      const next = enabled
        ? ALL_CODES.filter((code) => code === target || extraLocales.includes(code))
        : extraLocales.filter((code) => code !== target);
      window.localStorage.setItem(ENABLED_STORAGE_KEY, JSON.stringify(next));
      setExtraLocales(next);
      // Switching off the language currently being read would leave the user
      // looking at a locale with no way back to it.
      if (!enabled && locale === target) setLocale(DEFAULT_LOCALE);
    };

    const t = (key: TranslationKey, vars?: Record<string, string | number>) => {
      // Entries are authored with mn and en; the other five locales are filled
      // in progressively, so a lookup is widened to "this locale, maybe".
      const entry = dictionary[key] as (Partial<Record<Locale, string>> & { en: string }) | undefined;
      // Overlay first: that is where the generated and reviewed translations
      // for the optional languages live. Then the entry's own locale, then the
      // English source term rather than the key, as gettext does — an
      // untranslated screen reads as English, not as plumbing.
      let text: string = overlays[locale]?.[key] || (entry ? entry[locale] || entry.en : key);
      if (vars) {
        for (const [name, replacement] of Object.entries(vars)) {
          text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(replacement));
        }
      }
      return text;
    };

    return { locale, setLocale, availableLocales, setLocaleEnabled, t };
  }, [locale, extraLocales]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}
