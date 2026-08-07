"use client";

/**
 * The kit every /module/* screen is built from.
 *
 * These screens are contributed by installed apps and all have the same shape:
 * a titled header, panels, an empty state, an error line, chips and a confirm
 * dialog. Sharing them keeps a warehouse list and an audit table looking like
 * one product rather than eight.
 */

import { useState } from "react";
import { AlertTriangle, Check, Copy, Loader2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export { useAccess, ReadOnlyNote } from "@/lib/permissions";

export function Screen({ icon, title, subtitle, action, children }: {
  icon: React.ReactNode; title: string; subtitle: string;
  action?: React.ReactNode; children: React.ReactNode;
}) {
  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="w-10 h-10 rounded-xl bg-indigo-50 text-indigo-600 grid place-content-center shrink-0">
            {icon}
          </span>
          <div>
            <h1 className="text-2xl font-bold text-slate-900">{title}</h1>
            <p className="text-sm text-slate-500 mt-0.5 max-w-2xl">{subtitle}</p>
          </div>
        </div>
        {action}
      </header>
      {children}
    </div>
  );
}

export function Panel({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <section className={`bg-white border border-slate-200 rounded-xl ${className}`}>{children}</section>;
}

export function Empty({ icon, children }: { icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="p-12 text-center border border-dashed border-slate-300 rounded-xl bg-white">
      <span className="text-slate-300 grid place-content-center mb-3">{icon}</span>
      <p className="text-sm text-slate-500 max-w-md mx-auto">{children}</p>
    </div>
  );
}

export function Loading({ label }: { label: string }) {
  return (
    <p className="p-12 text-center text-slate-400 flex items-center justify-center gap-2">
      <Loader2 className="w-4 h-4 animate-spin" /> {label}
    </p>
  );
}

export function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p className="text-sm text-rose-700 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2 flex items-center gap-2">
      <AlertTriangle className="w-4 h-4 shrink-0" /> {children}
    </p>
  );
}

export function Chip({ children, tone = "slate", mono }: {
  children: React.ReactNode; tone?: "slate" | "blue" | "amber" | "emerald" | "rose"; mono?: boolean;
}) {
  const tones = {
    slate: "bg-slate-100 text-slate-600",
    blue: "bg-blue-50 text-blue-700",
    amber: "bg-amber-50 text-amber-700 border border-amber-200",
    emerald: "bg-emerald-50 text-emerald-700",
    rose: "bg-rose-50 text-rose-700 border border-rose-200",
  };
  return (
    <span className={`text-[11px] px-2 py-0.5 rounded ${tones[tone]} ${mono ? "font-mono" : ""} break-all`}>
      {children}
    </span>
  );
}

/** useCopy tracks which value was last copied, so one button can show a tick. */
export function useCopy() {
  const [copied, setCopied] = useState("");
  return {
    copied,
    copy(value: string, id: string) {
      void navigator.clipboard.writeText(value);
      setCopied(id);
      setTimeout(() => setCopied(""), 2000);
    },
  };
}

export function CopyButton({ value, id, copied, onCopy }: {
  value: string; id: string; copied: string; onCopy: (value: string, id: string) => void;
}) {
  return (
    <button onClick={() => onCopy(value, id)} className="shrink-0 text-slate-400 hover:text-slate-700" aria-label="copy">
      {copied === id ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
    </button>
  );
}

export function Modal({ children, onClose }: { children: React.ReactNode; onClose: () => void }) {
  return (
    <div className="fixed inset-0 bg-slate-900/60 backdrop-blur-sm flex items-start justify-center z-50 p-4 overflow-y-auto">
      <div className="bg-white rounded-xl w-full max-w-md p-6 shadow-2xl my-8 relative">
        <button onClick={onClose} className="absolute top-4 right-4 text-slate-400 hover:text-slate-600" aria-label="close">
          <X className="w-4 h-4" />
        </button>
        {children}
      </div>
    </div>
  );
}

export function ConfirmDialog({ title, body, confirmLabel, danger, onCancel, onConfirm }: {
  title: string; body: string; confirmLabel: string; danger?: boolean;
  onCancel: () => void; onConfirm: () => void;
}) {
  const { t } = useI18n();
  return (
    <Modal onClose={onCancel}>
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-slate-900 flex items-center gap-2">
          <AlertTriangle className={`w-5 h-5 ${danger ? "text-rose-600" : "text-amber-600"}`} />
          {title}
        </h2>
        <p className="text-sm text-slate-600">{body}</p>
        <div className="flex justify-end gap-2">
          <button onClick={onCancel} className="px-4 py-2 text-sm text-slate-600 hover:bg-slate-100 rounded-lg">
            {t("base.action.cancel")}
          </button>
          <button
            onClick={onConfirm}
            className={`px-4 py-2 text-sm text-white rounded-lg font-semibold ${danger ? "bg-rose-600 hover:bg-rose-700" : "bg-amber-600 hover:bg-amber-700"}`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </Modal>
  );
}

/** relative renders a timestamp as "3 days ago", or a fallback when absent. */
export function relativeDate(value: string | undefined, fallback: string, locale: string) {
  if (!value) return fallback;
  const then = new Date(value).getTime();
  const days = Math.round((then - Date.now()) / 86_400_000);
  if (Number.isNaN(days)) return fallback;
  const rtf = new Intl.RelativeTimeFormat(locale === "mn" ? "mn" : "en", { numeric: "auto" });
  if (Math.abs(days) < 1) return rtf.format(Math.round((then - Date.now()) / 3_600_000), "hour");
  return rtf.format(days, "day");
}


