"use client";

import React from "react";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import TaskQueue from "@/components/gov/TaskQueue";
import { PageHeader } from "@/components/ui";
import { CalendarClock, ChevronRight, Inbox, Landmark, Timer, Workflow } from "lucide-react";

/**
 * App root. The blueprint menu splits the app into five screens; this page is
 * the way in — counters and the live queue on top, shortcuts to the rest.
 * It keeps the /gov-services path working for anyone who bookmarked the queue.
 */
const SHORTCUTS = [
  { id: "requests", href: "/module/gov-services/requests", Icon: Inbox },
  { id: "appointments", href: "/module/gov-services/appointments", Icon: CalendarClock },
  { id: "catalog", href: "/module/gov-services/catalog", Icon: Landmark },
  { id: "workflow", href: "/module/gov-services/workflow", Icon: Workflow },
  { id: "sla", href: "/module/gov-services/sla", Icon: Timer },
] as const;

export default function GovServicesPage() {
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Landmark className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.view.title")}
        subtitle={t("gov.view.subtitle")}
      />

      <TaskQueue />

      <section className="grid sm:grid-cols-2 xl:grid-cols-3 gap-3">
        {SHORTCUTS.map(({ id, href, Icon }) => (
          <Link
            key={id}
            href={href}
            className="group p-4 bg-white border border-slate-200 rounded-xl hover:border-[var(--gerege-blue)] transition-colors"
          >
            <div className="flex items-start gap-3">
              <Icon className="w-5 h-5 text-[var(--gerege-blue)] shrink-0 mt-0.5" />
              <div className="flex-1 min-w-0">
                <div className="font-semibold text-slate-800">{t(`gov.menu.${id}` as never)}</div>
                <p className="text-xs text-slate-500 mt-1 leading-snug">{t(`gov.view.${id}_hint` as never)}</p>
              </div>
              <ChevronRight className="w-4 h-4 text-slate-300 group-hover:text-[var(--gerege-blue)]" />
            </div>
          </Link>
        ))}
      </section>
    </div>
  );
}
