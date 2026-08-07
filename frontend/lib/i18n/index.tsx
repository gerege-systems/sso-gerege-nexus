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

export type Locale = "mn" | "en";

export const LOCALES: { code: Locale; label: string; flag: string }[] = [
  { code: "mn", label: "Монгол", flag: "/icons/flag-mn.png" },
  { code: "en", label: "English", flag: "/icons/flag-en.png" },
];

const STORAGE_KEY = "locale";
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
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | null>(null);

function isLocale(value: string | null): value is Locale {
  return value === "mn" || value === "en";
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  // Server and first client render must agree, so the stored preference is
  // applied in an effect rather than during initial state.
  const [locale, setLocaleState] = useState<Locale>(DEFAULT_LOCALE);

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (isLocale(stored)) {
      setLocaleState(stored);
    }
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo<I18nValue>(() => {
    const setLocale = (next: Locale) => {
      window.localStorage.setItem(STORAGE_KEY, next);
      setLocaleState(next);
    };

    const t = (key: TranslationKey, vars?: Record<string, string | number>) => {
      const entry = dictionary[key];
      // Fall back to the English source term rather than the key, as gettext
      // does: an untranslated screen reads as English, not as plumbing.
      let text: string = entry ? entry[locale] || entry.en : key;
      if (vars) {
        for (const [name, replacement] of Object.entries(vars)) {
          text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(replacement));
        }
      }
      return text;
    };

    return { locale, setLocale, t };
  }, [locale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}
