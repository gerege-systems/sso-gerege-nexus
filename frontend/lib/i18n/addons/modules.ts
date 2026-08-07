/**
 * modules — the app-contributed screens under /module/*.
 *
 * Each of these reads data the app already stores; there is no copy here for a
 * feature that does not exist.
 */
export const modules = {
  // Contacts
  "mod.contacts.segments.title": { mn: "Сегментүүд", en: "Segments" },
  "mod.contacts.segments.subtitle": { mn: "Байгууллага болон төлвөөр нь бүлэглэсэн харилцагчид", en: "Your contacts grouped by company and status" },
  "mod.contacts.segments.by_company": { mn: "Байгууллагаар", en: "By company" },
  "mod.contacts.segments.no_company": { mn: "(байгууллага заагаагүй)", en: "(no company)" },
  "mod.contacts.segments.active": { mn: "Идэвхтэй", en: "Active" },
  "mod.contacts.segments.inactive": { mn: "Идэвхгүй", en: "Inactive" },
  "mod.contacts.segments.reachable": { mn: "Холбогдох боломжтой", en: "Reachable" },
  "mod.contacts.segments.reachable_hint": { mn: "и-мэйл эсвэл утастай", en: "has an email or a phone" },
  "mod.contacts.segments.unreachable_hint": { mn: "хоёулаа хоосон — эдгээрт хүрэх арга алга", en: "both blank — there is no way to reach these" },

  "mod.contacts.duplicates.title": { mn: "Давхардал", en: "Duplicates" },
  "mod.contacts.duplicates.subtitle": { mn: "И-мэйл, утас эсвэл нэрээрээ давхцаж буй бичлэгүүд", en: "Records that collide on email, phone or name" },
  "mod.contacts.duplicates.none": { mn: "Давхардал олдсонгүй.", en: "No duplicates found." },
  "mod.contacts.duplicates.by": { mn: "Давхцсан талбар", en: "Collides on" },
  "mod.contacts.duplicates.count": { mn: "{n} бичлэг", en: "{n} records" },
  "mod.contacts.duplicates.note": { mn: "Энэ дэлгэц юуг ч нэгтгэхгүй — зөвхөн заана. Аль нь үлдэхийг та Харилцагчид хэсгээс шийднэ.", en: "This screen merges nothing: it only points. Which record survives is your call, on the Contacts screen." },

  "mod.contacts.import.title": { mn: "Импорт", en: "Import contacts" },
  "mod.contacts.import.subtitle": { mn: "CSV файлаас харилцагч оруулах", en: "Bring contacts in from a CSV file" },
  "mod.contacts.import.drop": { mn: "CSV файл сонгох", en: "Choose a CSV file" },
  "mod.contacts.import.expected": { mn: "Хүлээгдэж буй баганууд", en: "Expected columns" },
  "mod.contacts.import.header_note": { mn: "Эхний мөр нь толгой байх ёстой. Баганы нэрийг таних тул дараалал чөлөөтэй.", en: "The first row must be a header. Columns are matched by name, so their order is free." },
  "mod.contacts.import.preview": { mn: "Урьдчилан харах", en: "Preview" },
  "mod.contacts.import.rows_ready": { mn: "{n} мөр оруулахад бэлэн", en: "{n} rows ready to import" },
  "mod.contacts.import.rows_skipped": { mn: "{n} мөр алгасагдана (нэр хоосон)", en: "{n} rows will be skipped (no name)" },
  "mod.contacts.import.run": { mn: "Импортлох", en: "Import" },
  "mod.contacts.import.done": { mn: "{ok} оруулсан, {failed} амжилтгүй", en: "{ok} imported, {failed} failed" },
  "mod.contacts.import.name_required": { mn: "name багана заавал байх ёстой.", en: "A name column is required." },

  // Inventory
  "mod.inventory.warehouses.title": { mn: "Агуулах", en: "Warehouses" },
  "mod.inventory.warehouses.subtitle": { mn: "Нөөц хадгалагдах байршлууд", en: "The locations your stock sits in" },
  "mod.inventory.warehouses.add": { mn: "Агуулах нэмэх", en: "Add warehouse" },
  "mod.inventory.warehouses.none": { mn: "Агуулах бүртгэгдээгүй байна. Нөөцийн бичилт агуулахад холбогддог тул эхлээд нэгийг үүсгэнэ.", en: "No warehouses yet. Stock is held against a warehouse, so start with one." },
  "mod.inventory.warehouses.code": { mn: "Код", en: "Code" },
  "mod.inventory.warehouses.address": { mn: "Хаяг", en: "Address" },
  "mod.inventory.warehouses.lines": { mn: "Нөөцийн мөр", en: "Stock lines" },

  "mod.inventory.replenishment.title": { mn: "Нөхөн дүүргэлт", en: "Replenishment" },
  "mod.inventory.replenishment.subtitle": { mn: "Заасан хэмжээнээс доош унасан нөөц", en: "Stock that has fallen below the level you set" },
  "mod.inventory.replenishment.threshold": { mn: "Босго", en: "Threshold" },
  "mod.inventory.replenishment.ok": { mn: "Энэ босгоос доош унасан нөөц алга.", en: "Nothing is below this threshold." },
  "mod.inventory.replenishment.out": { mn: "Дууссан", en: "Out of stock" },
  "mod.inventory.replenishment.low": { mn: "Бага", en: "Low" },
  "mod.inventory.replenishment.qty": { mn: "Үлдэгдэл", en: "On hand" },
  "mod.inventory.replenishment.note": { mn: "Босго нь энэ дэлгэцийн шүүлтүүр бөгөөд хадгалагддаггүй — бараа тус бүрийн дахин захиалгын цэг хараахан байхгүй.", en: "The threshold filters this screen and is not stored: there is no per-product reorder point yet." },

  // Billing
  "mod.billing.reports.title": { mn: "Орлогын тайлан", en: "Revenue reports" },
  "mod.billing.reports.subtitle": { mn: "Нэхэмжлэхээс гаргасан нийлбэр", en: "Totals drawn from your invoices" },
  "mod.billing.reports.total": { mn: "Нийт дүн", en: "Total invoiced" },
  "mod.billing.reports.vat": { mn: "НӨАТ", en: "VAT" },
  "mod.billing.reports.count": { mn: "Нэхэмжлэх", en: "Invoices" },
  "mod.billing.reports.by_status": { mn: "Төлвөөр", en: "By status" },
  "mod.billing.reports.by_month": { mn: "Сараар", en: "By month" },
  "mod.billing.reports.by_ebarimt": { mn: "e-Barimt төлөв", en: "e-Barimt status" },
  "mod.billing.reports.none": { mn: "Нэхэмжлэх алга.", en: "No invoices yet." },

  // Documents
  "mod.documents.approvals.title": { mn: "Батлах дараалал", en: "Approval queue" },
  "mod.documents.approvals.subtitle": { mn: "Батлагдаагүй хүлээгдэж буй баримтууд", en: "Documents still waiting on someone" },
  "mod.documents.approvals.none": { mn: "Хүлээгдэж буй баримт алга.", en: "Nothing is waiting." },
  "mod.documents.approvals.waiting": { mn: "Хүлээгдэж буй", en: "Waiting" },
  "mod.documents.approvals.settled": { mn: "Шийдэгдсэн", en: "Settled" },
  "mod.documents.approvals.signed_by": { mn: "Гарын үсэг зурсан", en: "Signed by" },
  "mod.documents.approvals.age": { mn: "Хугацаа", en: "Waiting for" },
  "mod.documents.approvals.note": { mn: "DRAFT болон PENDING төлөвтэй баримтыг хүлээгдэж буй гэж үзнэ. Батлах үйлдэл нь Баримт хэсэгт байна.", en: "DRAFT and PENDING count as waiting. Acting on one is done from the Documents screen." },

  // E-Sign
  "mod.esign.logs.title": { mn: "Гарын үсгийн лог", en: "Signature logs" },
  "mod.esign.logs.subtitle": { mn: "Тоон гарын үсгийн шалгалт ба баталгаажуулалтын түүх", en: "Certificate checks and signings, in order" },
  "mod.esign.logs.none": { mn: "Бичилт алга.", en: "Nothing logged yet." },
  "mod.esign.logs.action": { mn: "Үйлдэл", en: "Action" },
  "mod.esign.logs.signer": { mn: "Гарын үсэг зурагч", en: "Signer" },
  "mod.esign.logs.document": { mn: "Баримт", en: "Document" },
  "mod.esign.logs.when": { mn: "Хэзээ", en: "When" },
  "mod.esign.logs.masked_note": { mn: "Регистр, утасны дугаарыг хэсэгчлэн далдалсан — энэ бол аудитын түүх, лавлах бүртгэл биш.", en: "Registration and phone numbers are partly masked: this is an audit trail, not a directory." },
} as const;
