/**
 * web — The client shell — sidebar, header, user menu and the placeholder a
 * menu falls back to before its screen exists.
 */
export const web = {
  "web.menu.app_store": { mn: "Апп Дэлгүүр", en: "App Store" },
  "web.menu.installed_apps": { mn: "Суулгасан аппууд", en: "Installed Apps" },
  "web.menu.integrations": { mn: "Интеграцууд", en: "Integrations" },
  "web.menu.settings": { mn: "Тохиргоо", en: "Settings" },
  "web.menu.appearance": { mn: "Харагдац", en: "Appearance" },
  "web.menu.preferences": { mn: "Тохиргоо", en: "Preferences" },
  "web.menu.ai_settings": { mn: "AI тохиргоо", en: "AI settings" },
  "web.menu.email_verification": { mn: "И-мэйл баталгаажуулалт", en: "Email verification" },

  "web.group.modules": { mn: "Модулиуд", en: "Modules" },
  "web.group.settings": { mn: "Тохиргоо", en: "Settings" },

  "web.field.theme": { mn: "Загвар", en: "Theme" },

  "web.label.platform": { mn: "Платформ", en: "Platform" },
  "web.label.apps": { mn: "Аппууд", en: "Apps" },

  "web.action.logout": { mn: "Гарах", en: "Sign out" },
  "web.action.close_menu": { mn: "Цэс хаах", en: "Close menu" },
  "web.action.toggle_menu": { mn: "Цэс нээх, хаах", en: "Toggle menu" },
  "web.action.more": { mn: "Бусад", en: "More" },
  "web.action.close_more": { mn: "Бусад аппыг хаах", en: "Close more apps" },
  "web.action.expand_all": { mn: "Бүгдийг нээх", en: "Expand all" },
  "web.action.collapse_all": { mn: "Бүгдийг хаах", en: "Collapse all" },

  "web.action.switch_tenant": { mn: "Байгууллага солих", en: "Switch organisation" },

  "web.view.tenants": { mn: "Байгууллагууд", en: "Organisations" },
  "web.view.more_apps": { mn: "Бусад апп", en: "More apps" },
  "web.view.search_placeholder": { mn: "Апп, цэс хайх...", en: "Search apps and menus..." },
  "web.view.coming_soon": { mn: "Удахгүй", en: "Coming soon" },
  "web.view.coming_soon_body": { mn: "Энэ хэсэг хөгжүүлэлтийн шатанд байна. Бэлэн болмогц энд харагдана.", en: "This screen is still being built. It will appear here once it ships." },

  "web.message.loading_platform": { mn: "Платформыг ачаалж байна...", en: "Loading Gerege SSO..." },
  "web.message.only_tenant": { mn: "Та зөвхөн энэ байгууллагад харьяалагдаж байна.", en: "You belong to this organisation only." },
  "web.message.tenant_switch_failed": { mn: "Байгууллага солиж чадсангүй. Дахин оролдоно уу.", en: "Could not switch organisation. Please try again." },
} as const;
