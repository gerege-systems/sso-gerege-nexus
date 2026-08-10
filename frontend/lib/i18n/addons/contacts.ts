/**
 * contacts — Customer and vendor directory.
 */
export const contacts = {
  "contacts.view.title": { mn: "Харилцагчийн бүртгэл", en: "Contacts Directory" },
  "contacts.view.create_title": { mn: "Шинэ харилцагч үүсгэх", en: "Create New Contact" },

  "contacts.action.create": { mn: "Шинэ харилцагч", en: "New Contact" },
  "contacts.action.xyp_autofill": { mn: "ХУР / XYP Auto-fill", en: "XYP auto-fill" },

  "contacts.message.loading": { mn: "Харилцагчдыг ачаалж байна...", en: "Loading contacts..." },
  "contacts.message.xyp_verified": { mn: "ХУР Баталгаажсан Иргэн", en: "XYP verified citizen" },

  "contacts.view.subtitle": { mn: "Байгууллагын харилцагч, үйлчлүүлэгч, түншүүдийн бүртгэл", en: "Manage business contacts, customers, and partners" },

  "contacts.field.full_name": { mn: "Бүтэн нэр", en: "Full Name" },

  "contacts.action.save": { mn: "Харилцагч хадгалах", en: "Save Contact" },

  // Empty states name no button. The label they used to quote — "New Contact" —
  // is itself translated, so the sentence went stale in every language the
  // moment the button was renamed, and quoted an English word besides.
  "contacts.message.empty": { mn: "Одоогоор харилцагч бүртгэгдээгүй байна. Эхний бүртгэлээ нэмнэ үү.", en: "No contacts created yet — add your first record." },
  "contacts.message.load_failed": { mn: "Харилцагчдыг ачаалж чадсангүй. Харилцагчид апп суулгасан, идэвхтэй эсэхийг шалгана уу.", en: "Failed to load contacts. Check that the Contacts app is installed and enabled." },
  "contacts.message.create_failed": { mn: "Харилцагч үүсгэж чадсангүй", en: "Failed to create contact" },
  "contacts.message.xyp_failed": { mn: "ХУР-аас лавлагаа авч чадсангүй", en: "XYP query failed" },
} as const;
