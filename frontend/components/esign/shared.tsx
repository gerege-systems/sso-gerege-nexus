"use client";

import React from "react";
import { AlertTriangle, CheckCircle2, Loader2, X } from "lucide-react";
import { EsignApiError, type BatchItemStatus, type BatchStatus, type LogOutcome, type SessionState } from "@/lib/esign";
import { useI18n } from "@/lib/i18n";

/**
 * Turns any thrown value into a message, without the machine code.
 *
 * Prefer useErrorMessage: the API answers in English, so this raw form puts an
 * English sentence in a Mongolian interface at exactly the moment something has
 * gone wrong. It remains for non-React callers and as the fallback when a code
 * has no translation yet.
 */
export function describeError(err: unknown, fallback: string): string {
  if (err instanceof EsignApiError) return err.message;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

/** The backend's machine code for a thrown value, if it carried one. */
export function errorCode(err: unknown): string | null {
  return err instanceof EsignApiError && err.code !== "UNKNOWN" ? err.code : null;
}

/**
 * Translates a backend failure through the dictionary, keyed by its machine
 * code. An untranslated code falls back to the server's own message, so a new
 * code added on the server still says something useful.
 */
export function useErrorMessage() {
  const { t } = useI18n();
  return (err: unknown, fallback?: string): string => {
    const code = errorCode(err);
    if (code) {
      const key = `esign.error.${code}`;
      const translated = t(key as never);
      if (translated !== key) return translated;
    }
    return describeError(err, fallback ?? t("base.message.error"));
  };
}

export function PageHeader({
  icon,
  title,
  subtitle,
  actions,
}: {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
          {icon}
          {title}
        </h1>
        <p className="text-sm text-slate-500 mt-1">{subtitle}</p>
      </div>
      {actions}
    </header>
  );
}

export function Banner({
  tone,
  message,
  onDismiss,
}: {
  tone: "error" | "success" | "info";
  message: string;
  onDismiss?: () => void;
}) {
  const { t } = useI18n();
  const style =
    tone === "error"
      ? "bg-red-50 border-red-200 text-red-700"
      : tone === "success"
        ? "bg-emerald-50 border-emerald-200 text-emerald-700"
        : "bg-blue-50 border-blue-200 text-blue-700";
  return (
    <div className={`p-3 border text-sm rounded-lg flex items-start gap-2 ${style}`}>
      {tone === "error" ? (
        <AlertTriangle className="w-4 h-4 mt-0.5 shrink-0" />
      ) : (
        <CheckCircle2 className="w-4 h-4 mt-0.5 shrink-0" />
      )}
      <span className="flex-1">{message}</span>
      {onDismiss && (
        <button onClick={onDismiss} aria-label={t("base.action.close")}>
          <X className="w-4 h-4" />
        </button>
      )}
    </div>
  );
}

export function Loading({ label }: { label?: string }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-2 text-slate-500 text-sm">
      <Loader2 className="w-4 h-4 animate-spin" />
      {label || t("base.message.loading")}
    </div>
  );
}

export function EmptyState({ message }: { message: string }) {
  return <p className="p-6 text-sm text-slate-500 text-center italic">{message}</p>;
}

export function Card({ title, children, actions }: { title?: string; children: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <section className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
      {(title || actions) && (
        <header className="px-4 py-3 border-b border-slate-200 flex items-center justify-between gap-3">
          {title && <h2 className="text-sm font-bold text-slate-800">{title}</h2>}
          {actions}
        </header>
      )}
      {children}
    </section>
  );
}

const SESSION_STYLE: Record<SessionState, string> = {
  pending: "bg-amber-50 text-amber-700 border-amber-200",
  completed: "bg-emerald-50 text-emerald-700 border-emerald-200",
  failed: "bg-red-50 text-red-700 border-red-200",
  expired: "bg-slate-100 text-slate-600 border-slate-200",
  rejected: "bg-rose-50 text-rose-700 border-rose-200",
};

const BATCH_STYLE: Record<BatchStatus, string> = {
  DRAFT: "bg-slate-100 text-slate-600 border-slate-200",
  RUNNING: "bg-blue-50 text-blue-700 border-blue-200",
  COMPLETED: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  CANCELLED: "bg-slate-100 text-slate-500 border-slate-200",
};

const ITEM_STYLE: Record<BatchItemStatus, string> = {
  PENDING: "bg-slate-100 text-slate-600 border-slate-200",
  RUNNING: "bg-blue-50 text-blue-700 border-blue-200",
  SIGNED: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  SKIPPED: "bg-slate-100 text-slate-500 border-slate-200",
};

const OUTCOME_STYLE: Record<LogOutcome, string> = {
  OK: "bg-emerald-50 text-emerald-700 border-emerald-200",
  FAILED: "bg-red-50 text-red-700 border-red-200",
  REJECTED: "bg-rose-50 text-rose-700 border-rose-200",
  EXPIRED: "bg-slate-100 text-slate-600 border-slate-200",
  CANCELLED: "bg-slate-100 text-slate-500 border-slate-200",
  UNVERIFIED: "bg-amber-50 text-amber-700 border-amber-200",
};

export function Badge({ tone, children }: { tone: string; children: React.ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1 text-[11px] font-bold px-2.5 py-0.5 rounded-full border ${tone}`}>
      {children}
    </span>
  );
}

/**
 * Translates a server enum through the dictionary, falling back to the raw
 * value. A rail added on the server before the dictionary catches up should
 * still render as something readable rather than a missing-key placeholder.
 */
function useEnumLabel(prefix: string) {
  const { t } = useI18n();
  return (value: string) => {
    const key = `${prefix}.${value.toLowerCase()}`;
    const label = t(key as never);
    return label === key ? value : label;
  };
}

export function SessionBadge({ state }: { state: SessionState }) {
  const label = useEnumLabel("esign.session");
  return <Badge tone={SESSION_STYLE[state] ?? SESSION_STYLE.pending}>{label(state)}</Badge>;
}

export function BatchBadge({ status }: { status: BatchStatus }) {
  const label = useEnumLabel("esign.batch");
  return <Badge tone={BATCH_STYLE[status] ?? BATCH_STYLE.DRAFT}>{label(status)}</Badge>;
}

export function ItemBadge({ status }: { status: BatchItemStatus }) {
  const label = useEnumLabel("esign.item");
  return <Badge tone={ITEM_STYLE[status] ?? ITEM_STYLE.PENDING}>{label(status)}</Badge>;
}

export function OutcomeBadge({ outcome }: { outcome: LogOutcome }) {
  const label = useEnumLabel("esign.outcome");
  return <Badge tone={OUTCOME_STYLE[outcome] ?? OUTCOME_STYLE.OK}>{label(outcome)}</Badge>;
}

/**
 * Page controls for a listing. It renders nothing when everything fits on one
 * page, so a short log does not carry dead chrome.
 */
export function Pager({
  total,
  offset,
  pageSize,
  onPage,
}: {
  total: number;
  offset: number;
  pageSize: number;
  onPage: (offset: number) => void;
}) {
  const { t } = useI18n();
  if (total <= pageSize) return null;

  const page = Math.floor(offset / pageSize) + 1;
  const pages = Math.ceil(total / pageSize);

  return (
    <nav className="flex items-center justify-between text-xs text-slate-500">
      <span>{t("base.message.page_summary", { page, pages, total })}</span>
      <div className="flex gap-2">
        <button
          onClick={() => onPage(Math.max(0, offset - pageSize))}
          disabled={offset === 0}
          className="px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 disabled:opacity-40 font-medium"
        >
          {t("base.action.previous")}
        </button>
        <button
          onClick={() => onPage(offset + pageSize)}
          disabled={offset + pageSize >= total}
          className="px-3 py-1.5 rounded-lg bg-slate-100 hover:bg-slate-200 disabled:opacity-40 font-medium"
        >
          {t("base.action.next")}
        </button>
      </div>
    </nav>
  );
}

/** Renders a byte count the way a person reads a file size. */
export function formatBytes(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** A short, monospaced fingerprint. The full SHA-256 is unreadable in a table. */
export function shortHash(hash?: string): string {
  if (!hash) return "—";
  return `${hash.slice(0, 8)}…${hash.slice(-6)}`;
}
