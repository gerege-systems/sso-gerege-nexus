/**
 * auth — Signing in through eID Mongolia, plus the administrator's password
 * fallback.
 */
export const auth = {
  "auth.view.eyebrow": { mn: "ҮНДЭСНИЙ ЦАХИМ ТАНИЛТ", en: "NATIONAL DIGITAL IDENTITY" },
  // The headline highlights its middle phrase, so it is stored in parts
  // rather than as markup inside a translation.
  "auth.view.title_lead": { mn: "Баталгаатай identity.", en: "Verified identity." },
  "auth.view.title_highlight": { mn: "Нэг удаагийн", en: "Sign in" },
  "auth.view.title_tail": { mn: "нэвтрэлт.", en: "once." },
  "auth.view.lede": {
    mn: "eID Mongolia апп дээр хүсэлтийг зөвшөөрөхөд Gerege Nexus болон холбогдсон SSO аппууд таны баталгаажсан session-ийг ашиглана.",
    en: "Approve the request in the eID Mongolia app and Gerege Nexus — along with every connected SSO app — uses that verified session.",
  },
  "auth.view.point_push": { mn: "Регистрийн дугаараар push хүсэлт", en: "Push request by registration number" },
  "auth.view.point_qr": { mn: "QR болон mobile App2App", en: "QR and mobile App2App" },
  "auth.view.point_rbac": { mn: "Tenant RBAC ба audit хамгаалалт", en: "Tenant RBAC and audit protection" },

  "auth.eid.title": { mn: "eID Mongolia", en: "eID Mongolia" },
  "auth.eid.subtitle": { mn: "Үндэсний цахим үнэмлэхээр баталгаажна", en: "Verified by the national digital ID" },
  "auth.eid.tab_id": { mn: "Регистрийн дугаар", en: "Registration number" },
  "auth.eid.tab_qr": { mn: "QR код", en: "QR code" },
  "auth.eid.reg_number": { mn: "Регистрийн дугаар", en: "Registration number" },
  "auth.eid.reg_number_placeholder": { mn: "АА00112233", en: "AA00112233" },
  "auth.eid.send_request": { mn: "eID апп руу хүсэлт илгээх", en: "Send a request to the eID app" },
  "auth.eid.verification_code": { mn: "Баталгаажуулах код", en: "Verification code" },
  "auth.eid.confirm_hint": { mn: "Апп дээрх кодтой тулгаад зөвшөөрнө үү", en: "Match the code shown in the app, then approve" },
  "auth.eid.footer": {
    mn: "Нууц үг дамжуулахгүй · Баталгаажуулалт eID апп дотор хийгдэнэ",
    en: "No password is sent · Approval happens inside the eID app",
  },

  "auth.action.retry": { mn: "Дахин оролдох", en: "Try again" },
  "auth.action.cancel": { mn: "Цуцлах", en: "Cancel" },
  "auth.action.admin_disclosure": { mn: "Системийн админ нэвтрэлт", en: "System administrator sign-in" },
  "auth.action.admin_sign_in": { mn: "Админаар нэвтрэх", en: "Sign in as administrator" },

  "auth.message.starting": { mn: "Хүсэлт үүсгэж байна…", en: "Creating the request…" },
  "auth.message.scan_qr": { mn: "eID Mongolia апп-аар QR кодыг уншуулна уу.", en: "Scan the QR code with the eID Mongolia app." },
  "auth.message.sent_push": { mn: "Таны eID Mongolia апп руу хүсэлт илгээлээ.", en: "A request has been sent to your eID Mongolia app." },
  "auth.message.expires_in": { mn: "Хүсэлтийн хугацаа: {time}", en: "Request expires in {time}" },
  "auth.message.expired": { mn: "Хүсэлтийн хугацаа дууслаа.", en: "The request has expired." },
  "auth.message.refused": { mn: "Та нэвтрэх хүсэлтийг татгалзлаа.", en: "You declined the sign-in request." },
  "auth.message.success": { mn: "Амжилттай. Систем рүү шилжиж байна…", en: "Signed in. Taking you to the platform…" },
  "auth.message.error_link": {
    mn: "eID баталгаажуулалтыг Gerege Nexus хэрэглэгчтэй холбож чадсангүй",
    en: "The eID verification could not be linked to a Gerege Nexus user",
  },
  "auth.message.error_service": {
    mn: "eID Mongolia үйлчилгээтэй холбогдож чадсангүй",
    en: "Could not reach the eID Mongolia service",
  },
  "auth.message.error_password": { mn: "Нэвтрэх боломжгүй байна", en: "Could not sign in" },
} as const;
