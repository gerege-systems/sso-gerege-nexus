/**
 * base — Terms every screen shares: field labels, generic actions and
 * system messages. Nothing here belongs to one application.
 */
export const base = {
  "base.field.name": { mn: "Нэр", en: "Name" },
  "base.field.email": { mn: "И-мэйл", en: "Email" },
  "base.field.phone": { mn: "Утас", en: "Phone" },
  "base.field.company": { mn: "Байгууллага", en: "Company" },
  "base.field.status": { mn: "Төлөв", en: "Status" },
  "base.field.type": { mn: "Төрөл", en: "Type" },
  "base.field.date": { mn: "Огноо", en: "Date" },
  "base.field.language": { mn: "Хэл", en: "Language" },
  "base.field.actions": { mn: "Үйлдэл", en: "Actions" },

  "base.action.save": { mn: "Хадгалах", en: "Save" },
  "base.action.create": { mn: "Үүсгэх", en: "Create" },
  "base.action.delete": { mn: "Устгах", en: "Delete" },
  "base.action.close": { mn: "Хаах", en: "Close" },
  "base.action.cancel": { mn: "Цуцлах", en: "Cancel" },
  "base.action.previous": { mn: "Өмнөх", en: "Previous" },
  "base.action.next": { mn: "Дараах", en: "Next" },

  "base.state.active": { mn: "Идэвхтэй", en: "Active" },
  "base.state.inactive": { mn: "Идэвхгүй", en: "Inactive" },

  "base.value.yes": { mn: "Тийм", en: "Yes" },
  "base.value.no": { mn: "Үгүй", en: "No" },

  "base.message.loading": { mn: "Ачаалж байна...", en: "Loading..." },
  "base.message.read_only": { mn: "Танд зөвхөн харах эрх бий. Өөрчлөлт хийхэд дараах эрх шаардана:", en: "You have read access only. Changing anything needs this permission:" },
  "base.message.admin_only_title": { mn: "Зөвхөн админд", en: "Administrators only" },
  "base.message.admin_only_body": { mn: "Энэ хэсгийг үзэхэд танай байгууллагын админ эрх шаардана. Тохиргоо → Хандалтын удирдлага хэсгээс админ эрх олгоно.", en: "This section needs tenant administrator rights. An administrator can grant them under Settings → Access control." },
  "base.message.no_data": { mn: "Одоогоор өгөгдөл алга.", en: "No data yet." },
  "base.message.error": { mn: "Алдаа гарлаа", en: "Something went wrong" },
  "base.message.page_of": { mn: "{page} / {total} хуудас", en: "Page {page} of {total}" },
} as const;
