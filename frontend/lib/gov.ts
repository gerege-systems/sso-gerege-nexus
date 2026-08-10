/**
 * Typed client for the configurable government service workflow
 * (io.example.gov_services). Every DTO here mirrors the Go structs in
 * backend/internal/apps/gov_services; nothing on this screen uses `any`.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export type FulfillmentMode = "LOCAL" | "DELEGATE" | "HYBRID";

export type TaskStatus =
  | "RECEIVED"
  | "ASSIGNED"
  | "IN_PROGRESS"
  | "FORWARDED"
  | "INFO_REQUESTED"
  | "COMPLETED"
  | "AWAITING_VERIFICATION"
  | "RETURNED"
  | "REJECTED"
  | "CLOSED"
  | "CANCELLED";

export type TaskAction =
  | "assign"
  | "start"
  | "delegate"
  | "request_info"
  | "complete"
  | "verify"
  | "return"
  | "reject"
  | "cancel"
  | "close";

export type RoutingStrategy = "SELF" | "PARENT" | "CHILD" | "SPECIFIC_UNIT" | "REGION_MATCH";

export interface OrgUnit {
  id: string;
  code: string;
  name: string;
  name_en?: string;
  unit_type: string;
  parent_id?: string | null;
  region_code?: string;
  active: boolean;
  created_at: string;
  children?: OrgUnit[];
}

export interface GovService {
  id: string;
  code: string;
  name: string;
  name_en?: string;
  category: string;
  description: string;
  fee: number;
  duration_days: number;
  required_evidences: string[];
  appointment_required: boolean;
  active: boolean;
  fulfillment_mode: FulfillmentMode;
  workflow_version_id?: string | null;
  owner_unit_id?: string | null;
}

export interface WorkflowStep {
  id: string;
  code: string;
  name: string;
  step_order: number;
  step_type: string;
  executor_rule: RoutingStrategy;
  sla_hours: number;
  requires_verification: boolean;
  is_terminal: boolean;
}

export interface WorkflowVersion {
  id: string;
  workflow_id: string;
  version: number;
  status: "DRAFT" | "PUBLISHED" | "ARCHIVED";
  published_at?: string | null;
  published_by?: string;
  steps?: WorkflowStep[];
}

export interface Workflow {
  id: string;
  code: string;
  name: string;
  name_en?: string;
  description?: string;
  versions?: WorkflowVersion[];
}

export interface WorkflowTemplate {
  Code: string;
  Name: string;
  NameEN: string;
  Description: string;
  Mode: FulfillmentMode;
}

export interface RoutingRule {
  id: string;
  service_id?: string | null;
  priority: number;
  match_field?: string;
  match_value?: string;
  strategy: RoutingStrategy;
  target_unit_id?: string | null;
  active: boolean;
  created_at: string;
}

export interface Task {
  id: string;
  application_id: string;
  parent_task_id?: string | null;
  unit_id: string;
  unit_name?: string;
  status: TaskStatus;
  step_code?: string;
  due_at?: string | null;
  overdue: boolean;
  result_code?: string;
  result_description?: string;
  completed_by?: string;
  verified_by?: string;
  row_version: number;
  created_at: string;
  reference?: string;
  service_name?: string;
}

export interface TimelineEvent {
  id: string;
  event_type: string;
  message: string;
  actor: string;
  from_status?: string;
  to_status?: string;
  unit_name?: string;
  created_at: string;
}

export interface RequestSummary {
  id: string;
  reference: string;
  applicant_name: string;
  status: string;
  service_name: string;
  fulfillment_mode: FulfillmentMode;
  source_system: string;
  current_unit_id?: string | null;
  created_at: string;
}

export interface RequestDetail {
  request: RequestSummary;
  tasks: Task[];
  timeline: TimelineEvent[];
}

export interface Appointment {
  id: string;
  application_id?: string | null;
  service_id: string;
  service_name?: string;
  citizen_name: string;
  scheduled_at: string;
  location: string;
  status: string;
  created_at: string;
  /** IN_PERSON or ONLINE. An online appointment carries a link, not an address. */
  mode: "IN_PERSON" | "ONLINE";
  meeting_url?: string;
  meeting_provider?: string;
  /** Why an online booking has no link yet. The slot is still booked. */
  meeting_error?: string;
}

export interface Dashboard {
  scope: string;
  unit_count: number;
  received: number;
  in_progress: number;
  delegated: number;
  awaiting_verification: number;
  info_requested: number;
  returned: number;
  completed: number;
  closed: number;
  rejected: number;
  cancelled: number;
  overdue: number;
  total_active: number;
  total_requests: number;
  unfulfilled_requests: number;
  fulfilled_requests: number;
  verification_pending_upper: number;
}

export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface TaskQuery {
  status?: string;
  unit_id?: string;
  service_id?: string;
  overdue?: boolean;
  page?: number;
  page_size?: number;
}

/** An error carrying the backend's stable machine code. */
export class GovApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "GovApiError";
    this.code = code;
    this.status = status;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept-Language": locale,
    ...(init.headers as Record<string, string>),
  };
  const res = await fetch(`${API_BASE}${path}`, { ...init, headers, credentials: "include" });
  if (!res.ok) {
    let message = "Request failed";
    let code = "UNKNOWN";
    try {
      const body = (await res.json()) as { error?: string; code?: string };
      message = body.error || message;
      code = body.code || code;
    } catch {
      // a non-JSON error body leaves the defaults in place
    }
    throw new GovApiError(message, code, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

const toQuery = (params: Record<string, string | number | boolean | undefined>) => {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "" && value !== false) search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
};

export const gov = {
  // Reference data
  services: () => request<GovService[]>("/gov/services"),
  units: () => request<OrgUnit[]>("/gov/units"),
  unitTree: () => request<OrgUnit[]>("/gov/units/tree"),
  workflows: () => request<Workflow[]>("/gov/workflows"),
  templates: () => request<WorkflowTemplate[]>("/gov/workflow-templates"),
  /** The list endpoint omits steps, so step-level SLA needs the version by id. */
  workflowVersion: (id: string) => request<WorkflowVersion>(`/gov/workflow-versions/${id}`),

  // Operations
  dashboard: () => request<Dashboard>("/gov/dashboard"),
  tasks: (query: TaskQuery = {}) => request<Page<Task>>(`/gov/tasks${toQuery({ ...query })}`),
  requestDetail: (id: string) => request<RequestDetail>(`/gov/requests/${id}`),

  act: (taskId: string, body: { action: TaskAction; row_version: number; comment?: string; target_unit_id?: string; result_code?: string }) =>
    request<Task>(`/gov/tasks/${taskId}/actions`, { method: "POST", body: JSON.stringify(body) }),

  ingest: (body: {
    source_system: string;
    external_request_id: string;
    service_code: string;
    applicant_name: string;
    applicant_reg?: string;
    fields?: Record<string, string>;
  }) => request<{ request: { id: string; reference: string }; created: boolean }>("/gov/requests/ingest", {
    method: "POST",
    body: JSON.stringify(body),
  }),

  appointments: () => request<Appointment[]>("/gov/appointments"),

  // mode: "ONLINE" asks the connected conferencing provider for a joining
  // link. The appointment is booked either way — a provider outage must not
  // cost the citizen their slot — so check meeting_error on the answer.
  bookAppointment: (body: {
    service_id: string;
    citizen_name: string;
    scheduled_at: string;
    location?: string;
    mode?: "IN_PERSON" | "ONLINE";
    duration_minutes?: number;
    application_id?: string | null;
  }) => request<Appointment>("/gov/appointments", { method: "POST", body: JSON.stringify(body) }),

  requests: (query: TaskQuery = {}) => request<Page<Task>>(`/gov/tasks${toQuery({ ...query })}`),

  // Configuration
  createService: (body: {
    code: string;
    name: string;
    category?: string;
    description?: string;
    fee?: number;
    duration_days?: number;
    required_evidences?: string[];
    appointment_required?: boolean;
  }) => request<GovService>("/gov/services", { method: "POST", body: JSON.stringify(body) }),

  routingRules: () => request<RoutingRule[]>("/gov/routing-rules"),

  createRoutingRule: (body: {
    strategy: RoutingStrategy;
    service_id?: string | null;
    priority?: number;
    match_field?: string;
    match_value?: string;
    target_unit_id?: string | null;
  }) => request<RoutingRule>("/gov/routing-rules", { method: "POST", body: JSON.stringify(body) }),

  createUnit: (body: { code: string; name: string; unit_type?: string; parent_id?: string | null; region_code?: string }) =>
    request<OrgUnit>("/gov/units", { method: "POST", body: JSON.stringify(body) }),

  createWorkflow: (body: { template: string; code?: string; name?: string }) =>
    request<WorkflowVersion>("/gov/workflows", { method: "POST", body: JSON.stringify(body) }),

  publishVersion: (id: string) =>
    request<{ status: string; id: string }>(`/gov/workflow-versions/${id}/publish`, { method: "POST" }),

  configureService: (id: string, body: { fulfillment_mode: FulfillmentMode; workflow_version_id?: string | null; owner_unit_id?: string | null }) =>
    request<{ status: string; id: string }>(`/gov/services/${id}/configuration`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
};

/** Actions the UI may offer for a status. The backend re-validates every one. */
export function availableActions(status: TaskStatus): TaskAction[] {
  switch (status) {
    case "RECEIVED":
      return ["assign", "start", "delegate", "reject"];
    case "ASSIGNED":
      return ["start", "delegate", "request_info", "complete", "reject"];
    case "IN_PROGRESS":
      return ["delegate", "request_info", "complete", "reject"];
    case "INFO_REQUESTED":
      return ["start", "reject"];
    case "RETURNED":
      return ["start", "complete", "reject"];
    case "AWAITING_VERIFICATION":
      return ["verify", "return"];
    case "COMPLETED":
      return ["close"];
    default:
      return [];
  }
}

/** Permission each action needs, so the UI hides what the user cannot do. */
export const ACTION_PERMISSION: Record<TaskAction, string> = {
  assign: "gov.process",
  start: "gov.process",
  delegate: "gov.delegate",
  request_info: "gov.process",
  complete: "gov.process",
  verify: "gov.verify",
  return: "gov.verify",
  reject: "gov.process",
  cancel: "gov.apply",
  close: "gov.process",
};

/** Actions the backend refuses without a comment. */
export const ACTION_REQUIRES_COMMENT: TaskAction[] = ["reject", "return", "request_info"];
