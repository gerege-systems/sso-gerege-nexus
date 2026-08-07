"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { AdminOnly, useAccess } from "@/lib/permissions";
import { BrainCircuit, BookOpen, Save, Plus } from "lucide-react";

type Prompt = { key: string; content: string; active: boolean; global: boolean };
type Knowledge = { id: string; title: string; content: string; source_url: string; updated_at: string };

export default function AISettings() {
  const { t } = useI18n();
  // Prompts and the knowledge base are administrator-only server-side; without
  // this the screen loaded, failed, and printed the raw 403 string as a notice.
  const { loading: checking, isAdmin } = useAccess();
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [knowledge, setKnowledge] = useState<Knowledge[]>([]);
  const [notice, setNotice] = useState("");
  const [draft, setDraft] = useState({ title: "", content: "", source_url: "" });

  async function load() {
    try {
      const [p, k] = await Promise.all([api.getAIPrompts(), api.getAIKnowledge()]);
      const merged = new Map<string, Prompt>();
      p.forEach((item) => merged.set(item.key, item));
      setPrompts([...merged.values()]);
      setKnowledge(k);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : t("base.message.error"));
    }
  }

  useEffect(() => { void load(); }, []);

  async function save(prompt: Prompt) {
    await api.updateAIPrompt(prompt.key, prompt.content, prompt.active);
    setNotice(t("ai.message.prompt_saved"));
  }

  async function add() {
    if (!draft.title.trim() || !draft.content.trim()) return;
    await api.createAIKnowledge(draft);
    setDraft({ title: "", content: "", source_url: "" });
    setNotice(t("ai.message.knowledge_added"));
    await load();
  }

  const heading = (
    <header>
      <p className="text-xs font-semibold uppercase tracking-widest text-[var(--gerege-blue)]">Gemini AI</p>
      <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2"><BrainCircuit />{t("ai.view.settings_title")}</h1>
      <p className="text-sm text-slate-500">{t("ai.view.settings_subtitle")}</p>
    </header>
  );

  if (!checking && !isAdmin) {
    return <div className="w-full space-y-6">{heading}<AdminOnly /></div>;
  }

  return (
    <div className="w-full space-y-6">
      {heading}

      {notice && <div className="rounded-lg bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] p-3 text-sm">{notice}</div>}

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,.65fr)] gap-6 items-start">
        <section className="bg-white border border-slate-200 rounded-xl p-5 lg:p-6 space-y-5 shadow-sm min-w-0">
          <h2 className="font-bold">{t("ai.view.system_prompt")}</h2>
          {prompts.map((prompt, index) => (
            <div key={prompt.key}>
              <div className="flex justify-between mb-1.5">
                <label className="text-sm font-semibold">{prompt.key}</label>
                <span className="text-xs text-slate-400">{t(prompt.global ? "ai.scope.global" : "ai.scope.tenant")}</span>
              </div>
              <textarea rows={7} value={prompt.content} onChange={(event) => setPrompts((items) => items.map((item, itemIndex) => itemIndex === index ? { ...item, content: event.target.value, global: false } : item))} className="w-full border border-slate-200 rounded-lg p-3 text-sm bg-white" />
              <button onClick={() => void save(prompt)} className="mt-2 bg-[var(--gerege-blue)] hover:bg-[var(--gerege-blue-hover)] text-white rounded-lg px-3 py-2 text-sm flex gap-2"><Save className="w-4 h-4" />{t("base.action.save")}</button>
            </div>
          ))}
        </section>

        <section className="bg-white border border-slate-200 rounded-xl p-5 lg:p-6 space-y-4 shadow-sm min-w-0 xl:sticky xl:top-24">
          <h2 className="font-bold flex gap-2"><BookOpen className="w-5 h-5" />{t("ai.view.knowledge")}</h2>
          <div className="grid gap-3">
            <input placeholder={t("ai.field.knowledge_title")} value={draft.title} onChange={(event) => setDraft({ ...draft, title: event.target.value })} className="border border-slate-200 rounded-lg p-2.5" />
            <input placeholder={t("ai.field.source_url")} value={draft.source_url} onChange={(event) => setDraft({ ...draft, source_url: event.target.value })} className="border border-slate-200 rounded-lg p-2.5" />
            <textarea placeholder={t("ai.field.knowledge_content")} value={draft.content} onChange={(event) => setDraft({ ...draft, content: event.target.value })} rows={8} className="border border-slate-200 rounded-lg p-2.5" />
          </div>
          <button onClick={() => void add()} className="bg-[var(--gerege-blue)] hover:bg-[var(--gerege-blue-hover)] text-white rounded-lg px-3 py-2 text-sm flex gap-2"><Plus className="w-4 h-4" />{t("ai.action.add_knowledge")}</button>
          <div className="divide-y max-h-72 overflow-y-auto">
            {knowledge.map((item) => <article key={item.id} className="py-3"><h3 className="font-semibold text-sm">{item.title}</h3><p className="text-xs text-slate-500 line-clamp-2">{item.content}</p></article>)}
          </div>
        </section>
      </div>
    </div>
  );
}
