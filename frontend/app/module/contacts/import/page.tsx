"use client";

/**
 * CSV contact import.
 *
 * Runs entirely against the existing POST /contacts endpoint — one request per
 * row — so there is no import pipeline to go wrong and no half-written batch to
 * clean up. Rows are previewed before anything is sent, and a row that fails
 * is reported rather than dropped.
 */

import { useMemo, useState } from "react";
import { CheckCircle2, FileUp, Upload, XCircle } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, ErrorNote, Panel, Screen } from "@/components/module/kit";

type Row = { name: string; email: string; phone: string; company: string; active: boolean };

const COLUMNS = ["name", "email", "phone", "company", "active"] as const;

/** parseCSV handles quoted fields and embedded commas; nothing fancier. */
function parseCSV(text: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = "";
  let quoted = false;

  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (quoted) {
      if (char === '"' && text[i + 1] === '"') { cell += '"'; i++; }
      else if (char === '"') quoted = false;
      else cell += char;
      continue;
    }
    if (char === '"') quoted = true;
    else if (char === ",") { row.push(cell); cell = ""; }
    else if (char === "\n") { row.push(cell); rows.push(row); row = []; cell = ""; }
    else if (char !== "\r") cell += char;
  }
  if (cell || row.length) { row.push(cell); rows.push(row); }
  return rows.filter((r) => r.some((c) => c.trim()));
}

export default function ContactImportPage() {
  const { t } = useI18n();
  const [rows, setRows] = useState<Row[]>([]);
  const [skipped, setSkipped] = useState(0);
  const [error, setError] = useState("");
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<{ ok: number; failed: number } | null>(null);

  function read(file: File) {
    setError("");
    setResult(null);
    const reader = new FileReader();
    reader.onload = () => {
      const table = parseCSV(String(reader.result));
      if (table.length === 0) return;

      // Columns are matched by header name, so the order in the file is free.
      const header = table[0].map((h) => h.trim().toLowerCase());
      if (!header.includes("name")) {
        setRows([]);
        setError(t("mod.contacts.import.name_required"));
        return;
      }
      const index = Object.fromEntries(COLUMNS.map((c) => [c, header.indexOf(c)])) as Record<string, number>;

      const parsed: Row[] = [];
      let dropped = 0;
      for (const line of table.slice(1)) {
        const pick = (column: string) => (index[column] >= 0 ? (line[index[column]] ?? "").trim() : "");
        const name = pick("name");
        if (!name) { dropped++; continue; }
        const active = pick("active").toLowerCase();
        parsed.push({
          name,
          email: pick("email"),
          phone: pick("phone"),
          company: pick("company"),
          // Absent means active; only an explicit false turns it off.
          active: !["false", "0", "no", "үгүй"].includes(active),
        });
      }
      setRows(parsed);
      setSkipped(dropped);
    };
    reader.readAsText(file);
  }

  async function run() {
    setRunning(true);
    setError("");
    let ok = 0;
    let failed = 0;
    for (const row of rows) {
      try {
        await api.createContact(row);
        ok++;
      } catch {
        failed++;
      }
    }
    setResult({ ok, failed });
    setRows([]);
    setRunning(false);
  }

  const preview = useMemo(() => rows.slice(0, 8), [rows]);

  return (
    <Screen icon={<Upload className="w-5 h-5" />} title={t("mod.contacts.import.title")} subtitle={t("mod.contacts.import.subtitle")}>
      <Panel className="p-5">
        <p className="text-xs font-semibold text-slate-700">{t("mod.contacts.import.expected")}</p>
        <div className="flex flex-wrap gap-1 mt-1.5">
          {COLUMNS.map((column) => (
            <Chip key={column} mono tone={column === "name" ? "blue" : "slate"}>
              {column}{column === "name" ? " *" : ""}
            </Chip>
          ))}
        </div>
        <p className="text-[11px] text-slate-400 mt-2">{t("mod.contacts.import.header_note")}</p>

        <label className="mt-4 flex items-center justify-center gap-2 border border-dashed border-slate-300 rounded-lg py-6 cursor-pointer hover:border-indigo-400 hover:bg-indigo-50/40 text-sm text-slate-600">
          <FileUp className="w-4 h-4" />
          {t("mod.contacts.import.drop")}
          <input
            type="file"
            accept=".csv,text/csv"
            className="sr-only"
            onChange={(e) => { const file = e.target.files?.[0]; if (file) read(file); }}
          />
        </label>
      </Panel>

      {error && <ErrorNote>{error}</ErrorNote>}

      {result && (
        <p className={`text-sm rounded-lg px-3 py-2 border flex items-center gap-2 ${result.failed ? "text-amber-800 bg-amber-50 border-amber-200" : "text-emerald-700 bg-emerald-50 border-emerald-200"}`}>
          {result.failed ? <XCircle className="w-4 h-4" /> : <CheckCircle2 className="w-4 h-4" />}
          {t("mod.contacts.import.done", { ok: result.ok, failed: result.failed })}
        </p>
      )}

      {rows.length > 0 && (
        <Panel className="overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-100 flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-bold text-slate-900">{t("mod.contacts.import.preview")}</h2>
            <Chip tone="emerald">{t("mod.contacts.import.rows_ready", { n: rows.length })}</Chip>
            {skipped > 0 && <Chip tone="amber">{t("mod.contacts.import.rows_skipped", { n: skipped })}</Chip>}
            <button
              onClick={run}
              disabled={running}
              className="ml-auto bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-1.5 rounded-lg text-sm font-semibold disabled:opacity-50"
            >
              {t("mod.contacts.import.run")}
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-slate-50 text-left text-[11px] uppercase tracking-wide text-slate-500">
                <tr>{COLUMNS.map((c) => <th key={c} className="px-4 py-2 font-semibold">{c}</th>)}</tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {preview.map((row, index) => (
                  <tr key={index}>
                    <td className="px-4 py-2 font-medium text-slate-900">{row.name}</td>
                    <td className="px-4 py-2 text-slate-600">{row.email || "—"}</td>
                    <td className="px-4 py-2 text-slate-600">{row.phone || "—"}</td>
                    <td className="px-4 py-2 text-slate-600">{row.company || "—"}</td>
                    <td className="px-4 py-2">{row.active ? "✓" : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {rows.length > preview.length && (
            <p className="px-4 py-2 text-[11px] text-slate-400">+ {rows.length - preview.length}</p>
          )}
        </Panel>
      )}
    </Screen>
  );
}
