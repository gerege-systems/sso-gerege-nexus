/**
 * billing — Invoicing, VAT and e-Barimt receipts.
 */
export const billing = {
  "billing.view.title": { mn: "Нэхэмжлэх ба e-Barimt татварын баримт", en: "Public Billing & e-Barimt Tax Receipts" },
  "billing.view.subtitle": { mn: "Улсын тэмдэгтийн хураамж, нэхэмжлэх ба татварын баримт", en: "State fee invoices, billing, and tax receipts" },
  "billing.view.create_title": { mn: "Улсын хураамжийн нэхэмжлэх үүсгэх", en: "Create State Fee Invoice" },

  "billing.field.contact": { mn: "Харилцагч", en: "Contact / Client" },
  "billing.field.contact_placeholder": { mn: "e.g. Гэрэгэ Системс ХХК", en: "e.g. Gerege Systems LLC" },
  "billing.field.total": { mn: "Нийт дүн", en: "Total Amount" },
  "billing.field.payment_status": { mn: "Төлбөрийн төлөв", en: "Payment Status" },
  "billing.field.ebarimt_status": { mn: "e-Barimt төлөв", en: "e-Barimt Status" },

  "billing.action.create": { mn: "Нэхэмжлэх үүсгэх", en: "Create Invoice" },

  "billing.message.loading": { mn: "Нэхэмжлэхүүдийг ачаалж байна...", en: "Loading invoices..." },

  "billing.field.invoice_number": { mn: "Нэхэмжлэхийн дугаар", en: "Invoice #" },
  "billing.field.amount": { mn: "Нэхэмжлэхийн дүн (₮)", en: "Invoice Amount (₮)" },

  "billing.action.generate": { mn: "Нэхэмжлэх үүсгэх", en: "Generate Invoice" },

  "billing.message.create_failed": { mn: "Нэхэмжлэх үүсгэж чадсангүй", en: "Failed to create invoice" },
} as const;
