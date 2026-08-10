/**
 * app_store — The application catalogue and what a tenant has installed.
 */
export const app_store = {
  "app_store.view.title": { mn: "Платформын Апп Дэлгүүр", en: "Platform App Store" },
  "app_store.view.subtitle": { mn: "Компиляцын үеийн бизнес модулиудыг суулгаж, идэвхжүүлж, удирдана", en: "Install, enable, and manage compile-time business modules" },
  "app_store.view.installed_title": { mn: "Суулгасан аппуудын тохиргоо", en: "Installed Apps Settings" },
  "app_store.view.search_placeholder": { mn: "Апп хайх...", en: "Search apps..." },

  "app_store.field.requires": { mn: "Шаардлага: ", en: "Requires:" },
  "app_store.field.application_name": { mn: "Аппликейшны нэр", en: "Application Name" },
  "app_store.field.module_id": { mn: "Модулийн ID", en: "Module ID" },
  "app_store.field.installed_version": { mn: "Суулгасан хувилбар", en: "Installed Version" },
  "app_store.field.installed_date": { mn: "Суулгасан огноо", en: "Installed Date" },

  "app_store.action.enable": { mn: "Идэвхжүүлэх", en: "Enable App" },
  "app_store.action.disable": { mn: "Идэвхгүй болгох", en: "Disable App" },

  "app_store.state.installed": { mn: "Суулгасан ба идэвхтэй", en: "Installed & Enabled" },
  "app_store.state.disabled": { mn: "Идэвхгүй", en: "Disabled" },

  "app_store.filter.all": { mn: "Бүгд", en: "All" },

  "app_store.message.loading": { mn: "Апп каталог ачаалж байна...", en: "Loading apps catalog..." },
  "app_store.message.loading_installed": { mn: "Суулгасан аппуудыг ачаалж байна...", en: "Loading installed apps..." },
  "app_store.message.no_match": { mn: "Хайлтад тохирох апп олдсонгүй.", en: "No apps found matching your query." },

  "app_store.view.installed_subtitle": { mn: "Суулгасан модулиудыг удирдаж, төлөвийг хянаж, идэвхжүүлэх буюу идэвхгүй болгоно", en: "Manage installed tenant modules, check operational status, and enable or disable features" },

  "app_store.action.browse_store": { mn: "Апп Дэлгүүр рүү очих", en: "Go to the App Store" },

  // Two self-contained sentences rather than one split around a link. A
  // sentence cut in half translates badly: word order moves in Chinese and
  // reverses in Arabic, so the fragments no longer join up.
  "app_store.message.none_installed": { mn: "Энэ тенантад одоогоор апп суулгаагүй байна.", en: "No apps installed for this tenant yet." },
  "app_store.message.load_failed": { mn: "Аппын каталогийг ачаалж чадсангүй", en: "Failed to load the app catalog" },
  "app_store.message.action_failed": { mn: "Үйлдэл амжилтгүй боллоо", en: "Action failed" },
} as const;
