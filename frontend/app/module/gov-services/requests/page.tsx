"use client";

import TaskQueue from "@/components/gov/TaskQueue";
import { PageHeader } from "@/components/ui";
import { useI18n } from "@/lib/i18n";
import { Inbox } from "lucide-react";

/** Service requests: the operational queue every level works from. */
export default function GovRequestsPage() {
  const { t } = useI18n();
  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Inbox className="w-7 h-7 text-[var(--gerege-blue)]" />}
        title={t("gov.menu.requests")}
        subtitle={t("gov.view.requests_hint")}
      />
      <TaskQueue />
    </div>
  );
}
