const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

async function fetcher<T>(url: string, options: RequestInit = {}): Promise<T> {
  const token = typeof window !== "undefined" ? localStorage.getItem("session_token") : null;
  // Server-owned content (menu labels, app store copy) is translated by the
  // API, so every request carries the locale the user picked.
  const locale = typeof window !== "undefined" ? window.localStorage.getItem("locale") || "mn" : "mn";
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "Accept-Language": locale,
    ...(options.headers as Record<string, string>),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${url}`, {
    ...options,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    let errMessage = "Request failed";
    try {
      const errData = await res.json();
      errMessage = errData.error || errMessage;
    } catch {
      // ignore
    }
    throw new Error(errMessage);
  }

  return res.json();
}

export const APP_MENU_CHANGED_EVENT = "gerege:app-menu-changed";

async function mutateApp(url: string) {
  const result = await fetcher<{ status: string; app: string }>(url, { method: "POST" });
  // Layout lives above the App Store pages, so a route refresh does not
  // recreate it. Notify the mounted shell to refetch tenant menus immediately.
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(APP_MENU_CHANGED_EVENT, { detail: result }));
  }
  return result;
}

export const api = {
  // Auth
  login: (email: string, password: string) =>
    fetcher<{ token: string; user: any }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  loginWithEID: (code?: string, redirectURI?: string, regNumber?: string, otpCode?: string, authMethod?: string) =>
    fetcher<{ token: string; user: any; identity: any }>("/auth/eid/login", {
      method: "POST",
      body: JSON.stringify({ code, redirect_uri: redirectURI, reg_number: regNumber, otp_code: otpCode, auth_method: authMethod }),
    }),

  startEID: (callbackUrl = "") => fetcher<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start",{method:"POST",body:JSON.stringify({callbackUrl})}),
  startEIDByNationalID: (nationalId:string,callbackUrl = "") => fetcher<{session_id:string;device_link_url?:string;verification_code:string;expires_at:string}>("/auth/eid/start-id",{method:"POST",body:JSON.stringify({national_id:nationalId,callbackUrl})}),
  // The poll is a long poll the API holds open for up to 25s, so the caller
  // passes a signal to drop it the moment the citizen cancels or leaves.
  pollEID: (sessionId:string,signal?:AbortSignal) => fetcher<{state:string;token?:string;identity?:any}>("/auth/eid/poll",{method:"POST",body:JSON.stringify({session_id:sessionId}),signal}),

  loginWithDAN: (danToken?: string, regNumber?: string, otpCode?: string) =>
    fetcher<{ token: string; user: any; dan_profile: any }>("/auth/dan/login", {
      method: "POST",
      body: JSON.stringify({ dan_token: danToken, reg_number: regNumber, otp_code: otpCode }),
    }),

  logout: () => fetcher<{ status: string }>("/auth/logout", { method: "POST" }),

  // permissions carries the effective grant of every role the member holds; it
  // is empty for administrators, who bypass the check.
  getMe: () => fetcher<{ id: string; tenant_id: string; tenant_name: string; name: string; email: string; is_admin: boolean; permissions?: string[] }>("/auth/me"),

  getMenus: () => fetcher<Array<{ id: string; app_id?: string; app_name?: string; parent_id?: string; label: string; path?: string; icon: string; order: number }>>("/menus"),

  // Odoo-style tenant access control
  getAccessOverview: () => fetcher<{
    roles: Array<{ id:string; code:string; name:string; description:string; active:boolean; system:boolean; permissions:string[] }>;
    permissions: Array<{ code:string; name:string; description:string; app:string }>;
    members: Array<{ membership_id:string; user_id:string; name:string; email:string; is_admin:boolean; roles:string[] }>;
  }>("/admin/access/overview"),
  createRole: (data:{code:string;name:string;description:string}) => fetcher<{id:string}>("/admin/access/roles",{method:"POST",body:JSON.stringify(data)}),
  updateRole: (id:string,data:{name:string;description:string;active:boolean}) => fetcher(`/admin/access/roles/${id}`,{method:"PUT",body:JSON.stringify(data)}),
  deleteRole: (id:string) => fetcher<void>(`/admin/access/roles/${id}`,{method:"DELETE"}),
  setRolePermissions: (id:string,permissions:string[]) => fetcher(`/admin/access/roles/${id}/permissions`,{method:"PUT",body:JSON.stringify({permissions})}),
  setMembershipRoles: (id:string,roles:string[]) => fetcher(`/admin/access/memberships/${id}/roles`,{method:"PUT",body:JSON.stringify({roles})}),

  // Store
  getStoreApps: () =>
    fetcher<
      Array<{
        id: string;
        slug: string;
        name: string;
        description: string;
        icon_url: string;
        category: string;
        version: string;
        installed: boolean;
        enabled: boolean;
        manifest: any;
      }>
    >("/store/apps"),

  getInstalledApps: () =>
    fetcher<
      Array<{
        id: string;
        app_id: string;
        slug: string;
        name: string;
        installed_version: string;
        status: string;
        enabled: boolean;
        installed_at: string;
      }>
    >("/installed-apps"),

  installApp: (slug: string) => mutateApp(`/store/apps/${slug}/install`),

  enableApp: (slug: string) => mutateApp(`/store/apps/${slug}/enable`),

  disableApp: (slug: string) => mutateApp(`/store/apps/${slug}/disable`),

  // Contacts App
  getContacts: () =>
    fetcher<
      Array<{
        id: string;
        name: string;
        email: string;
        phone: string;
        company: string;
        active: boolean;
        created_at: string;
      }>
    >("/contacts"),

  createContact: (data: { name: string; email: string; phone: string; company: string; active: boolean }) =>
    fetcher("/contacts", { method: "POST", body: JSON.stringify(data) }),

  updateContact: (id: string, data: { name: string; email: string; phone: string; company: string; active: boolean }) =>
    fetcher(`/contacts/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  // Products App
  getProducts: () =>
    fetcher<
      Array<{
        id: string;
        sku: string;
        name: string;
        price: number;
        active: boolean;
        created_at: string;
      }>
    >("/products"),

  createProduct: (data: { sku: string; name: string; price: number; active: boolean }) =>
    fetcher("/products", { method: "POST", body: JSON.stringify(data) }),

  updateProduct: (id: string, data: { sku: string; name: string; price: number; active: boolean }) =>
    fetcher(`/products/${id}`, { method: "PUT", body: JSON.stringify(data) }),

  // Inventory App
  getWarehouses: () =>
    fetcher<
      Array<{
        id: string;
        code: string;
        name: string;
        address: string;
        created_at: string;
      }>
    >("/inventory/warehouses"),

  createWarehouse: (data: { code: string; name: string; address: string }) =>
    fetcher("/inventory/warehouses", { method: "POST", body: JSON.stringify(data) }),

  getStockLevels: () =>
    fetcher<
      Array<{
        id: string;
        warehouse_id: string;
        product_id: string;
        quantity: number;
        updated_at: string;
      }>
    >("/inventory/stock-levels"),

  getStockMovements: () =>
    fetcher<
      Array<{
        id: string;
        warehouse_id: string;
        product_id: string;
        quantity_change: number;
        reference: string;
        created_at: string;
      }>
    >("/inventory/movements"),

  adjustStock: (data: { warehouse_id: string; product_id: string; quantity_change: number; reference: string }) =>
    fetcher("/inventory/adjustments", { method: "POST", body: JSON.stringify(data) }),

  // AI Assistant & Forecasting
  queryAICopilot: (prompt: string) =>
    fetcher<{ answer: string; intent: string; data?: any; actionable?: string[] }>("/ai/copilot", {
      method: "POST",
      body: JSON.stringify({ prompt }),
    }),

  chatAI: (data: {
    prompt?: string;
    lang?: string;
    history?: Array<{ role: "user" | "model"; text: string }>;
    audio?: { mime: string; data: string };
  }) => fetcher<{ answer: string; reply: string; steps?: Array<{ tool: string }>; degraded?: boolean }>("/ai/chat", {
    method: "POST", body: JSON.stringify(data),
  }),

  speakAI: (text: string) => fetcher<{ mime: string; data: string }>("/ai/tts", {
    method: "POST", body: JSON.stringify({ text }),
  }),

  translateAI: (data: { text?: string; audio?: { mime: string; data: string }; target_lang: string; speak?: boolean }) =>
    fetcher<{ source_text: string; translated: string; audio?: { mime: string; data: string } }>("/ai/translate", {
      method: "POST", body: JSON.stringify(data),
    }),

  getAIPrompts: () => fetcher<Array<{key:string;content:string;active:boolean;global:boolean}>>("/admin/ai/prompts"),
  updateAIPrompt: (key:string, content:string, active=true) => fetcher(`/admin/ai/prompts/${key}`, {method:"PUT",body:JSON.stringify({content,active})}),
  getAIKnowledge: () => fetcher<Array<{id:string;title:string;content:string;source_url:string;updated_at:string}>>("/admin/ai/knowledge"),
  createAIKnowledge: (data:{title:string;content:string;source_url:string}) => fetcher<{id:string}>("/admin/ai/knowledge",{method:"POST",body:JSON.stringify(data)}),

  getAIForecast: () =>
    fetcher<
      Array<{
        product_id: string;
        sku: string;
        product_name: string;
        current_stock: number;
        recommended_min: number;
        reorder_alert: boolean;
        suggested_reorder: number;
      }>
    >("/ai/stock-forecast"),

  // XYP State Data Exchange (xyp.gerege.mn)
  queryXYPCitizen: (regNumber: string) =>
    fetcher<{
      reg_number: string;
      civil_id: string;
      last_name: string;
      first_name: string;
      gender: string;
      address: string;
      passport_status: string;
      verified: boolean;
    }>("/xyp/citizen", {
      method: "POST",
      body: JSON.stringify({ reg_number: regNumber }),
    }),

  queryXYPCompany: (companyReg: string) =>
    fetcher<{
      company_reg: string;
      name: string;
      executive: string;
      address: string;
      vat_payer: boolean;
      status: string;
      founding_date: string;
    }>("/xyp/company", {
      method: "POST",
      body: JSON.stringify({ company_reg: companyReg }),
    }),

  // External Integrations Manager
  getIntegrations: () =>
    fetcher<
      Array<{
        id: string;
        name: string;
        type: string;
        target_url: string;
        status: string;
        last_ping_at: string;
      }>
    >("/integrations"),

  registerIntegration: (data: { name: string; type: string; target_url: string; secret_key?: string }) =>
    fetcher("/integrations", { method: "POST", body: JSON.stringify(data) }),

  // Billing App (io.example.billing)
  getInvoices: () =>
    fetcher<
      Array<{
        id: string;
        invoice_number: string;
        contact_name: string;
        amount: number;
        vat_amount: number;
        ebarimt_status: string;
        status: string;
        created_at: string;
      }>
    >("/billing/invoices"),

  createInvoice: (data: { contact_name: string; amount: number }) =>
    fetcher("/billing/invoices", { method: "POST", body: JSON.stringify(data) }),

  // Documents App (io.example.documents)
  getDocuments: () =>
    fetcher<
      Array<{
        id: string;
        title: string;
        doc_type: string;
        status: string;
        signed_by: string;
        created_at: string;
      }>
    >("/documents"),

  createDocument: (data: { title: string; doc_type: string }) =>
    fetcher("/documents", { method: "POST", body: JSON.stringify(data) }),

  // PDF E-Sign App (io.example.esign)
  getEsignDocuments: () =>
    fetcher<
      Array<{
        id: string;
        title: string;
        file_name: string;
        status: string;
        page_count: number;
        signer_name: string;
        signer_reg_no: string;
        signer_phone: string;
        signed_at: string | null;
        created_at: string;
      }>
    >("/esign/documents"),

  uploadEsignDocument: (data: { title: string; file_name: string; pdf_base64: string }) =>
    fetcher("/esign/documents", { method: "POST", body: JSON.stringify(data) }),

  checkEsignCert: (data: { phone_no: string; civil_id: string; data?: string }) =>
    fetcher<{ is_valid: boolean; given_name: string; surname: string; common_name: string; uid: string }>(
      "/esign/cert/check",
      { method: "POST", body: JSON.stringify(data) }
    ),

  signEsignDocument: (
    id: string,
    data: { phone_no: string; signer_name: string; signer_reg_no: string; signature_image64: string }
  ) => fetcher<{ status: string; document_id: string; signed_at: string }>(`/esign/documents/${id}/sign`, {
    method: "POST",
    body: JSON.stringify(data),
  }),

  getEsignLogs: () =>
    fetcher<
      Array<{
        id: string;
        document_id: string;
        reg_no: string;
        phone_no: string;
        first_name: string;
        last_name: string;
        action: string;
        created_at: string;
      }>
    >("/esign/logs"),

  downloadEsignDocument: async (id: string, variant: "original" | "signed"): Promise<Blob> => {
    const token = typeof window !== "undefined" ? localStorage.getItem("session_token") : null;
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    const res = await fetch(`${API_BASE}/esign/documents/${id}/download?variant=${variant}`, {
      headers,
      credentials: "include",
    });
    if (!res.ok) throw new Error("Download failed");
    return res.blob();
  },

  // Developer Portal & OAuth2 SSO Apps
  //
  // client_secret comes back only from create and rotate-secret; every other
  // read omits it, because the server keeps a digest and cannot reproduce it.
  getDeveloperApps: () => fetcher<OAuth2Client[]>("/developer/apps"),
  getDeveloperApp: (clientID: string) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}`),
  createDeveloperApp: (app: OAuth2ClientDraft) =>
    fetcher<OAuth2Client>("/developer/apps", { method: "POST", body: JSON.stringify(app) }),
  updateDeveloperApp: (clientID: string, app: OAuth2ClientDraft) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}`, {
      method: "PUT",
      body: JSON.stringify(app),
    }),
  deleteDeveloperApp: (clientID: string) =>
    fetcher<void>(`/developer/apps/${encodeURIComponent(clientID)}`, { method: "DELETE" }),
  rotateDeveloperAppSecret: (clientID: string) =>
    fetcher<OAuth2Client>(`/developer/apps/${encodeURIComponent(clientID)}/rotate-secret`, {
      method: "POST",
    }),
  getDeveloperScopes: () =>
    fetcher<{ scopes: OAuth2Scope[]; grant_types: string[] }>("/developer/scopes"),
  getDeveloperEndpoints: () => fetcher<Record<string, string>>("/developer/endpoints"),

  // OAuth2 consent screen. The query string is the authorization request the
  // browser arrived with; the server re-validates all of it rather than
  // trusting what the page echoes back.
  getConsentPrompt: (query: string) => fetcher<ConsentPrompt>(`/oauth2/consent?${query}`),
  decideConsent: (query: string, approved: boolean) => {
    const form = new URLSearchParams(query);
    form.set("approved", String(approved));
    return fetcher<{ redirect_to: string }>("/oauth2/consent", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: form.toString(),
    });
  },
};

export type OAuth2Scope = {
  name: string;
  description: string;
  description_mn: string;
  sensitive?: boolean;
};

export type OAuth2ClientDraft = {
  client_name: string;
  client_uri?: string;
  client_type?: "confidential" | "public";
  redirect_uris: string[];
  grant_types: string[];
  scopes: string[];
  disabled?: boolean;
};

export type OAuth2Client = {
  id: string;
  client_id: string;
  client_name: string;
  client_uri?: string;
  client_type: "confidential" | "public";
  redirect_uris: string[];
  grant_types: string[];
  scopes: string[];
  disabled: boolean;
  created_at: string;
  updated_at: string;
  secret_rotated_at?: string;
  last_used_at?: string;
  /** Present only in the response that created or rotated it. */
  client_secret?: string;
};

export type ConsentPrompt = {
  client_id: string;
  client_name: string;
  client_uri?: string;
  logo_uri?: string;
  redirect_uri: string;
  scopes: OAuth2Scope[];
  already_granted: string[];
};
