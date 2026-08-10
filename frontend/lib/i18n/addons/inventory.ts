/**
 * inventory — Warehouses, stock levels and movements.
 */
export const inventory = {
  "inventory.view.title": { mn: "Агуулах ба нөөцийн үйл ажиллагаа", en: "Inventory & Warehouse Operations" },
  "inventory.view.warehouses": { mn: "Агуулахууд", en: "Warehouses" },
  "inventory.view.stock_levels": { mn: "Одоогийн үлдэгдэл", en: "Live Stock Levels" },
  "inventory.view.movements": { mn: "Хөдөлгөөний түүх", en: "Stock Movements History" },
  "inventory.view.adjustment": { mn: "Үлдэгдлийн тохируулга", en: "Stock Adjustment" },
  "inventory.view.create_warehouse": { mn: "Агуулах үүсгэх", en: "Create Warehouse" },

  "inventory.field.warehouse": { mn: "Агуулах", en: "Warehouse" },
  "inventory.field.product": { mn: "Бараа", en: "Product" },
  "inventory.field.address": { mn: "Хаяг", en: "Address" },
  "inventory.field.change": { mn: "Өөрчлөлт", en: "Change" },
  "inventory.field.available_quantity": { mn: "Боломжит тоо хэмжээ", en: "Available Quantity" },
  "inventory.field.reference": { mn: "Баримт / Шалтгаан", en: "Reference / Reason" },
  "inventory.field.reference_note": { mn: "Тайлбар", en: "Reference Note" },
  "inventory.field.reference_placeholder": { mn: "жишээ: PO-98421 эсвэл тооллогын зөрүү", en: "e.g. PO-98421 or Physical count adjustment" },
  "inventory.field.datetime": { mn: "Огноо, цаг", en: "Date & Time" },

  "inventory.action.create_warehouse": { mn: "Шинэ агуулах", en: "New Warehouse" },
  "inventory.action.adjust": { mn: "Тохируулга хийх", en: "Adjust Stock" },

  "inventory.message.loading": { mn: "Агуулахын мэдээлэл ачаалж байна...", en: "Loading inventory data..." },

  "inventory.view.subtitle": { mn: "Агуулах бүрийн үлдэгдлийг хянаж, тохируулгыг бүртгэнэ", en: "Track stock levels across warehouses and log adjustments" },

  "inventory.field.code": { mn: "Код", en: "Code" },
  "inventory.field.warehouse_name": { mn: "Агуулахын нэр", en: "Warehouse Name" },
  "inventory.field.select_warehouse": { mn: "Агуулах сонгох", en: "Select Warehouse" },
  "inventory.field.select_product": { mn: "Бараа сонгох", en: "Select Product" },

  "inventory.action.save_warehouse": { mn: "Агуулах хадгалах", en: "Save Warehouse" },
  "inventory.action.confirm_adjustment": { mn: "Тохируулга баталгаажуулах", en: "Confirm Adjustment" },

  "inventory.message.no_address": { mn: "Хаяг оруулаагүй", en: "No address specified" },
  "inventory.message.empty_warehouses": { mn: "Одоогоор агуулах тохируулаагүй байна. Эхний байршлаа нэмнэ үү.", en: "No warehouses configured yet — add your first location." },
  "inventory.message.empty_stock": { mn: "Одоогоор үлдэгдэл бүртгэгдээгүй байна. Тохируулга хийж орлогоо бүртгэнэ үү.", en: "No stock recorded yet — make an adjustment to book your first intake." },
  "inventory.message.empty_movements": { mn: "Хөдөлгөөний бүртгэл алга байна.", en: "No stock movement events logged yet." },
  "inventory.message.load_failed": { mn: "Агуулахын мэдээллийг ачаалж чадсангүй. Агуулах апп суулгасан, идэвхтэй эсэхийг шалгана уу.", en: "Failed to load inventory data. Check that the Inventory app is installed and enabled." },
  "inventory.message.warehouse_created": { mn: "Агуулах амжилттай үүслээ", en: "Warehouse created" },
  "inventory.message.warehouse_failed": { mn: "Агуулах үүсгэж чадсангүй", en: "Failed to create warehouse" },
  "inventory.message.adjustment_recorded": { mn: "Үлдэгдлийн тохируулга бүртгэгдлээ", en: "Stock adjustment recorded" },
  "inventory.message.adjustment_failed": { mn: "Үлдэгдлийн тохируулга амжилтгүй боллоо", en: "Stock adjustment failed" },
} as const;
