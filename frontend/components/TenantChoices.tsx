"use client";

import { useCallback, useEffect, useState } from "react";
import { Building2, Check } from "lucide-react";
import { api } from "@/lib/api";
import { resetAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";

export interface TenantOption {
  id: string;
  name: string;
  slug: string;
}

/**
 * The answer is cached the way lib/access caches the identity: two controls now
 * offer this list — the brand mark in the header and the account menu, which is
 * the only one of the two the mobile shell shows — and opening one after the
 * other should not ask the server the same question twice.
 */
let pending: Promise<TenantOption[]> | null = null;

/** Forget the cached list. Sign-out and a completed switch both invalidate it. */
export function forgetTenants() {
  pending = null;
}

/**
 * The tenants the signed-in person may act for, fetched the first time a
 * control that offers them is opened rather than with the shell: most people
 * hold one membership and will never open either.
 */
export function useTenants(active: boolean) {
  const [tenants, setTenants] = useState<TenantOption[] | null>(null);
  const [switching, setSwitching] = useState(false);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!active) return;
    let alive = true;
    // Never cache the failure: a rejected promise left in place would answer
    // every later opening with the error that belonged to one moment.
    const request = (pending ??= api
      .getTenants()
      .then((answer) => answer.tenants || [])
      .catch((error) => {
        pending = null;
        throw error;
      }));
    request
      .then((value) => alive && setTenants(value))
      .catch(() => alive && setTenants([]));
    return () => {
      alive = false;
    };
  }, [active]);

  const switchTo = useCallback(async (id: string) => {
    setSwitching(true);
    setFailed(false);
    try {
      await api.switchTenant(id);
      // Everything on screen was fetched for the tenant being left — the menus,
      // the permissions, every list on the page behind this control. A full
      // load is the only honest way to drop all of it at once, and /apps is
      // somewhere every tenant has, unlike the screen being stood on.
      resetAccess();
      forgetTenants();
      window.location.assign("/apps");
    } catch {
      setSwitching(false);
      setFailed(true);
    }
  }, []);

  return { tenants, switching, failed, switchTo };
}

/**
 * The rows themselves, so the header control and the account menu offer the
 * same list rather than two that drift. Each host supplies its own heading and
 * surrounding chrome.
 */
export function TenantChoices({
  current,
  tenants,
  switching,
  failed,
  onChoose,
  onStay,
}: {
  current?: string;
  tenants: TenantOption[] | null;
  switching: boolean;
  failed: boolean;
  onChoose: (id: string) => void;
  /** Called when the current tenant is picked — nothing to switch, so the host
   *  just closes. */
  onStay: () => void;
}) {
  const { t } = useI18n();

  return (
    <>
      {tenants === null && <p className="px-4 py-2 text-sm text-slate-500">{t("base.message.loading")}</p>}
      {tenants?.map((option) => (
        <button
          key={option.id}
          type="button"
          role="menuitem"
          disabled={switching}
          onClick={() => (option.id === current ? onStay() : onChoose(option.id))}
          className={`w-full flex items-center gap-3 rounded-lg px-4 py-2.5 text-left hover:bg-[var(--gerege-surface-2)] disabled:opacity-60 ${
            option.id === current ? "bg-[var(--gerege-blue-soft)]" : ""
          }`}
        >
          <span className={option.id === current ? "text-[var(--gerege-blue)]" : "text-slate-400"}>
            <Building2 className="w-4 h-4" />
          </span>
          <span className="min-w-0 flex-1">
            <strong className="block text-sm font-medium truncate">{option.name}</strong>
            <small className="block text-xs text-slate-500 truncate">{option.slug}</small>
          </span>
          {option.id === current && <Check className="w-4 h-4 shrink-0 text-[var(--gerege-blue)]" />}
        </button>
      ))}
      {tenants?.length === 1 && (
        <p className="px-4 pb-2 pt-1 text-xs text-slate-500">{t("web.message.only_tenant")}</p>
      )}
      {failed && (
        <p role="alert" className="px-4 pb-2 pt-1 text-xs text-rose-600">
          {t("web.message.tenant_switch_failed")}
        </p>
      )}
    </>
  );
}
