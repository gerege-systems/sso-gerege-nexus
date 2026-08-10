/**
 * products — Product catalogue: SKUs, names and pricing.
 */
export const products = {
  "products.view.title": { mn: "Барааны каталог", en: "Product Catalog" },
  "products.view.subtitle": { mn: "SKU, барааны нэр, үнийн удирдлага", en: "Manage SKUs, product names, and pricing" },
  "products.view.create_title": { mn: "Шинэ бараа үүсгэх", en: "Create New Product" },

  "products.field.name": { mn: "Барааны нэр", en: "Product Name" },
  "products.field.sku": { mn: "SKU код", en: "SKU" },
  "products.field.sku_placeholder": { mn: "жишээ: PROD-001", en: "e.g. PROD-001" },
  "products.field.price": { mn: "Нэгж үнэ", en: "Unit Price" },

  "products.action.create": { mn: "Шинэ бараа", en: "New Product" },

  "products.message.loading": { mn: "Бараануудыг ачаалж байна...", en: "Loading products..." },

  "products.action.save": { mn: "Бараа хадгалах", en: "Save Product" },

  "products.message.empty": { mn: "Одоогоор бараа нэмээгүй байна. Каталогоо эхлүүлэхийн тулд эхний бараагаа нэмнэ үү.", en: "No products added yet — add your first one to start the catalog." },
  "products.message.load_failed": { mn: "Барааг ачаалж чадсангүй. Бараа бүтээгдэхүүн апп суулгасан, идэвхтэй эсэхийг шалгана уу.", en: "Failed to load products. Check that the Products app is installed and enabled." },
  "products.message.create_failed": { mn: "Бараа үүсгэж чадсангүй", en: "Failed to create product" },
} as const;
