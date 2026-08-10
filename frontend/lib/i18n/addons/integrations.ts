/**
 * integrations — Connections and webhooks to external systems.
 */
export const integrations = {
  "integrations.view.title": { mn: "Гадаад системийн интеграц ба Webhook", en: "External System Integrations & Webhooks" },
  "integrations.view.create_title": { mn: "Интеграц холбогч бүртгэх", en: "Register Integration Connector" },

  "integrations.field.type": { mn: "Интеграцын төрөл", en: "Integration Type" },
  "integrations.field.name_placeholder": { mn: "жишээ: Борлуулалтын Webhook", en: "e.g. Sales Webhook" },
  "integrations.field.secret": { mn: "Нууц түлхүүр (гарын үсэг)", en: "Secret Key (Signing)" },
  "integrations.field.secret_placeholder": { mn: "HMAC нууц түлхүүр (заавал бус)", en: "Optional HMAC secret" },

  "integrations.type.webhook": { mn: "Webhook хүлээн авагч", en: "Webhook Listener" },
  "integrations.type.government_gateway": { mn: "Төрийн гарц", en: "Government Gateway" },
  "integrations.type.payment_gateway": { mn: "Төлбөрийн гарц", en: "Payment Gateway" },
  "integrations.type.custom_rest": { mn: "Захиалгат REST хаяг", en: "Custom REST Endpoint" },

  "integrations.action.create": { mn: "Интеграц нэмэх", en: "Add Integration" },

  "integrations.message.loading": { mn: "Интеграцуудыг ачаалж байна...", en: "Loading integrations..." },

  "integrations.view.subtitle": { mn: "Төрийн гарц, REST API холбогч, event webhook-уудыг удирдана", en: "Manage government gateways, REST API connectors, and event webhooks" },

  "integrations.field.name": { mn: "Интеграцын нэр", en: "Integration Name" },
  "integrations.field.target_url": { mn: "Хүлээн авах хаяг (URL)", en: "Target Endpoint URL" },

  "integrations.action.register": { mn: "Холбогч бүртгэх", en: "Register Connector" },

  "integrations.message.register_failed": { mn: "Интеграц бүртгэж чадсангүй", en: "Failed to register integration" },

  "integrations.type.google_drive": { mn: "Google Drive", en: "Google Drive" },
  "integrations.type.dropbox": { mn: "Dropbox", en: "Dropbox" },
  "integrations.type.google_meet": { mn: "Google Meet", en: "Google Meet" },

  "integrations.field.connected_account": { mn: "Холбогдсон бүртгэл", en: "Connected account" },
  "integrations.field.drive_folder": { mn: "Хадгалах хавтасны ID (заавал бус)", en: "Destination folder ID (optional)" },
  "integrations.field.drive_folder_placeholder": { mn: "хоосон бол Drive-ийн үндсэн хавтас", en: "empty means the Drive root" },
  "integrations.field.dropbox_folder": { mn: "Хадгалах хавтасны зам (заавал бус)", en: "Destination folder path (optional)" },
  "integrations.field.dropbox_folder_placeholder": { mn: "жишээ: Гэрээ/2026", en: "e.g. Contracts/2026" },
  "integrations.field.calendar_id": { mn: "Календарийн ID (заавал бус)", en: "Calendar ID (optional)" },
  "integrations.field.auto_export": { mn: "Гарын үсэг зурсан баримтыг энэ хаяг руу автоматаар хадгалах", en: "File every newly signed document here automatically" },

  "integrations.action.connect": { mn: "Холбох", en: "Connect" },
  "integrations.action.disconnect": { mn: "Салгах", en: "Disconnect" },

  "integrations.state.unavailable": { mn: "боломжгүй", en: "unavailable" },

  "integrations.message.empty": { mn: "Одоогоор интеграц бүртгээгүй байна.", en: "No integrations registered yet." },
  "integrations.message.not_connected": { mn: "Бүртгэл холбогдоогүй байна", en: "No account connected yet" },
  "integrations.message.connected": { mn: "{name} амжилттай холбогдлоо.", en: "{name} is connected." },
  "integrations.message.auto_export_on": { mn: "Автомат хадгалалт идэвхтэй", en: "Automatic filing is on" },
  // Saving a credential needs a key to seal it with. Saying so here beats
  // letting the save fail with the same sentence.
  "integrations.message.encryption_missing": { mn: "INTEGRATION_ENCRYPTION_KEY тохируулаагүй тул нууц түлхүүр, бүртгэлийн холболтыг хадгалах боломжгүй байна.", en: "INTEGRATION_ENCRYPTION_KEY is not set, so secrets and account connections cannot be stored." },
  // Meet links are issued by Calendar, not by a Meet API of its own. An
  // administrator who does not know that will look for the wrong permission.
  "integrations.message.meet_via_calendar": { mn: "Google Meet холбоосыг Календарийн арга хэрэглэн үүсгэдэг тул холболт нь календарийн зөвшөөрөл шаардана.", en: "A Meet link is issued through Calendar, so this connector asks for calendar access." },
  "integrations.message.connect_after_save": { mn: "Хадгалсны дараа “Холбох” дарж бүртгэлээ зөвшөөрнө үү.", en: "After saving, use Connect to authorise the account." },
  "integrations.message.confirm_delete": { mn: "{name} интеграцыг устгах уу?", en: "Delete the {name} integration?" },
  "integrations.message.load_failed": { mn: "Интеграцуудыг ачаалж чадсангүй", en: "Failed to load integrations" },
  "integrations.message.connect_failed": { mn: "Бүртгэлийг холбож чадсангүй", en: "Could not connect the account" },
  "integrations.message.disconnect_failed": { mn: "Бүртгэлийг салгаж чадсангүй", en: "Could not disconnect the account" },
  "integrations.message.delete_failed": { mn: "Интеграцыг устгаж чадсангүй", en: "Failed to delete the integration" },
} as const;
