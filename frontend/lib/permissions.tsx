"use client";

/**
 * Client-side permission checks, so a screen can shape itself around what the
 * caller may actually do.
 *
 * These are presentation only. Every endpoint behind them enforces the same
 * rule server-side — hiding a button is a courtesy to the user, never the
 * control.
 */

import { useEffect, useState } from "react";
import { Lock, ShieldAlert } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

type Access = { loading: boolean; allowed: boolean; isAdmin: boolean };

/**
 * useAccess resolves the signed-in user's rights.
 *
 * Administrators bypass the permission check server-side and come back with an
 * empty permission list, so the flag is honoured separately — otherwise an
 * admin would be shown a read-only screen.
 */
export function useAccess(permission?: string): Access {
  const [access, setAccess] = useState<Access>({ loading: true, allowed: false, isAdmin: false });

  useEffect(() => {
    void (async () => {
      try {
        const me = await api.getMe();
        setAccess({
          loading: false,
          isAdmin: me.is_admin,
          allowed: me.is_admin || !permission || (me.permissions ?? []).includes(permission),
        });
      } catch {
        setAccess({ loading: false, allowed: false, isAdmin: false });
      }
    })();
  }, [permission]);

  return access;
}

/**
 * ReadOnlyNote explains why the actions are missing.
 *
 * Without it a member holding only the read permission sees create, rotate and
 * delete controls and finds out they cannot use them from a raw 403 after the
 * click.
 */
export function ReadOnlyNote({ permission }: { permission: string }) {
  const { t } = useI18n();
  return (
    <p className="text-xs text-slate-500 bg-slate-100 border border-slate-200 rounded-lg px-3 py-2 flex items-start gap-2">
      <Lock className="w-3.5 h-3.5 shrink-0 mt-0.5" />
      <span>
        {t("base.message.read_only")} <code className="font-mono text-slate-600">{permission}</code>
      </span>
    </p>
  );
}

/**
 * AdminOnly replaces a whole screen the caller cannot use.
 *
 * The alternative these pages shipped with was worse in both directions: the
 * integrations screen swallowed the 403 and rendered an empty list, which
 * reads as "you have no integrations", and the AI screen surfaced the raw
 * server string.
 */
export function AdminOnly() {
  const { t } = useI18n();
  return (
    <div className="p-12 text-center border border-dashed border-slate-300 rounded-xl bg-white">
      <ShieldAlert className="w-9 h-9 text-slate-300 mx-auto mb-3" />
      <h2 className="font-bold text-slate-900">{t("base.message.admin_only_title")}</h2>
      <p className="text-sm text-slate-500 mt-1 max-w-md mx-auto">{t("base.message.admin_only_body")}</p>
    </div>
  );
}
