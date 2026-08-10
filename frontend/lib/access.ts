"use client";

import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";

export interface Identity {
  id: string;
  is_admin: boolean;
  permissions?: string[];
}

/**
 * The caller's effective grant, as decided by the tenant's roles. One request
 * per page load: the promise is cached at module level so several screens (or
 * several components on one screen) share the same answer.
 *
 * This only decides what to render. Every endpoint re-checks the permission on
 * the server, so hiding a button is convenience, never the control itself.
 */
let identity: Promise<Identity> | null = null;

/**
 * Forget the cached answer. Sign-out and sign-in both navigate on the client —
 * the module is never re-evaluated — so without this the cache outlives the
 * session that filled it: the next person to sign in at the same desk was shown
 * the previous one's screens, and a caller who signed in after an expired
 * session kept the failure and saw nothing they were entitled to.
 */
export function resetAccess() {
  identity = null;
}

export function useAccess() {
  const [me, setMe] = useState<Identity | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let alive = true;
    // Cache the request, but never the failure: a rejected promise left in
    // place answers every later caller with the error that belonged to one
    // moment. Clearing it on rejection lets the next mount ask again.
    const pending = (identity ??= api.getMe().catch((error) => {
      identity = null;
      throw error;
    }));
    pending
      .then((value) => alive && setMe(value))
      // A failure here means the session is gone; Layout redirects to /login.
      .catch(() => undefined)
      .finally(() => alive && setReady(true));
    return () => {
      alive = false;
    };
  }, []);

  const granted = useMemo(() => new Set(me?.permissions || []), [me]);

  return {
    ready,
    isAdmin: me?.is_admin === true,
    can: (code: string) => me?.is_admin === true || granted.has(code),
  };
}
