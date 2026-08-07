/**
 * website — The public landing page: what the platform is, before anyone
 * signs in.
 */
export const website = {
  "website.menu.features": { mn: "Боломжууд", en: "Capabilities" },
  "website.menu.trust": { mn: "Аюулгүй байдал", en: "Security" },
  "website.menu.technology": { mn: "Технологи", en: "Technology" },

  "website.action.sign_in": { mn: "Нэвтрэх", en: "Sign in" },
  "website.action.eid_sign_in": { mn: "eID-ээр нэвтрэх", en: "Sign in with eID" },
  "website.action.see_features": { mn: "Боломжийг үзэх", en: "See what it does" },

  // The hero headline is one sentence with a highlighted middle, so it is
  // stored in three parts rather than as markup inside a translation.
  "website.view.hero_title_lead": { mn: "Нэг нэвтрэлт —", en: "One sign-in —" },
  "website.view.hero_title_highlight": { mn: "таны бүх", en: "every app" },
  "website.view.hero_title_tail": { mn: "апп", en: "you have" },
  "website.view.hero_lede": {
    mn: "Gerege SSO нь үндэсний цахим үнэмлэхэд суурилсан нэвтрэлтийг OIDC/SSO чадвартай нэгтгэв. Иргэн нэг удаа баталгаажаад эрхтэй бүх аппдаа найдвартай орно.",
    en: "Gerege SSO joins national digital identity to an OIDC/SSO provider. A citizen verifies once and reaches every application they are entitled to.",
  },

  "website.stat.eid": { mn: "Баталгаат identity", en: "Verified identity" },
  "website.stat.standards": { mn: "Нээлттэй стандарт", en: "Open standards" },
  "website.stat.sso": { mn: "Нэг session", en: "One session" },

  "website.view.features_eyebrow": { mn: "GEREGE IDENTITY LAYER", en: "GEREGE IDENTITY LAYER" },
  "website.view.features_title": {
    mn: "Нэвтрэлт бол тусдаа дэлгэц биш, платформын суурь",
    en: "Sign-in is not a screen, it is the floor the platform stands on",
  },
  "website.view.features_lede": {
    mn: "Gerege Platform-ийн батлагдсан урсгалыг Gerege SSO-ийн tenant, role, audit болон SSO загварт нэгтгэлээ.",
    en: "The Gerege Platform's proven flow, folded into Gerege SSO's tenants, roles, audit trail and SSO model.",
  },

  "website.feature.instant_title": { mn: "Цахим үнэмлэхээр хормын дотор", en: "Digital ID in seconds" },
  "website.feature.instant_body": {
    mn: "Регистрийн дугаараар eID апп руу хүсэлт илгээх, компьютер дээр QR уншуулах, утсан дээр App2App холбоосоор нэвтэрнэ.",
    en: "Push a request to the eID app by registration number, scan a QR on the desktop, or hand off App2App on the phone.",
  },
  "website.feature.sso_title": { mn: "Нэг нэвтрэлт — олон систем", en: "One sign-in, many systems" },
  "website.feature.sso_body": {
    mn: "Платформын баталгаажсан session нь OAuth2/OIDC provider-тэй нэг trust boundary ашиглана. Холбогдсон апп бүр дахин нэвтрүүлэх шаардлагагүй.",
    en: "The platform session and the OAuth2/OIDC provider share one trust boundary, so no connected app asks again.",
  },
  "website.feature.passwordless_title": { mn: "Нууц үггүй, сервер талын хамгаалалт", en: "Passwordless, guarded server-side" },
  "website.feature.passwordless_body": {
    mn: "RP secret зөвхөн backend-д хадгалагдана. Browser-д identity credential ил гарахгүй, session token hash хэлбэрээр хадгалагдана.",
    en: "The RP secret never leaves the backend, no identity credential reaches the browser, and session tokens are stored hashed.",
  },
  "website.feature.channels_title": { mn: "Апп ба вэбийн нэг урсгал", en: "One flow across app and web" },
  "website.feature.channels_body": {
    mn: "Desktop cross-device, mobile same-device callback, push болон QR бүгд ижил start/poll contract-аар ажиллана.",
    en: "Desktop cross-device, mobile same-device callback, push and QR all run on the same start/poll contract.",
  },

  "website.view.trust_eyebrow": { mn: "ИДЭВХТЭЙ ХАМГААЛАЛТ", en: "ACTIVE PROTECTION" },
  "website.view.trust_title": {
    mn: "Танилтаас эрх хүртэл нэг баталгааны гинж",
    en: "One chain of proof, from identity to permission",
  },
  "website.view.trust_lede": {
    mn: "eID identity → серверийн session → tenant membership → RBAC → OIDC client. Алхам бүр сервер талд шалгагдана.",
    en: "eID identity → server session → tenant membership → RBAC → OIDC client. Every link is checked on the server.",
  },
  "website.trust.cookie": { mn: "httpOnly, SameSite session cookie", en: "httpOnly, SameSite session cookie" },
  "website.trust.rbac": { mn: "Tenant-аар тусгаарласан role ба permission", en: "Roles and permissions isolated per tenant" },
  "website.trust.allowlist": { mn: "RP callback origin allowlist", en: "RP callback origin allowlist" },
  "website.trust.audit": { mn: "Login ба access audit event", en: "Login and access audit events" },

  // Key kept as-is: it is internal, and renaming it would touch every caller
  // for no user-visible gain. The value is what reaches the screen.
  "website.tech.erp_body": { mn: "Gerege Nexus дээр суурилсан нэгдсэн нэвтрэлт, tenant тусгаарлалт, RBAC", en: "Single sign-on, tenant isolation and RBAC, built on Gerege Nexus" },
  "website.tech.eid_body": { mn: "Push, QR, App2App, баталгаажсан identity", en: "Push, QR, App2App, verified identity" },
  "website.tech.sso_body": { mn: "Холбогдсон аппууд, нэг session", en: "Connected applications, one session" },

  "website.message.footer_note": {
    mn: "eID-д суурилсан · Нээлттэй стандарт · Secure by design",
    en: "Built on eID · Open standards · Secure by design",
  },
} as const;
