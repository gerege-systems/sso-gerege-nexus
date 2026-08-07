/**
 * documents — Document routing, categories and e-signatures.
 */
export const documents = {
  "documents.view.title": { mn: "Цахим баримт ба тоон гарын үсэг", en: "Digital Documents & E-Signatures" },
  "documents.view.create_title": { mn: "Цахим баримт үүсгэх", en: "Create Digital Document" },
  "documents.view.subtitle": {
    mn: "Байгууллагын баримтын урсгал, тоон гарын үсэг ба батламжийн процесс.",
    en: "Enterprise document routing, digital signatures, and approval workflows.",
  },
  "documents.field.title_pattern_placeholder": {
    mn: "Хамтран ажиллах гэрээ {year}",
    en: "Partnership agreement {year}",
  },
  "documents.view.sign_title": { mn: "Баримтад гарын үсэг зурах", en: "Sign Document" },
  "documents.view.history_title": { mn: "Гарын үсгийн түүх", en: "Signature history" },
  "documents.view.approvals_hint": {
    mn: "Гарын үсэг хүлээж байгаа баримтууд — E-ID / ДАН-аар батлах эсвэл татгалзах.",
    en: "Documents awaiting a decision — approve with an E-ID / DAN signature, or reject.",
  },

  // Screen title for the queue. The sidebar entry itself is labelled by the
  // menu blueprint on the server; this is the heading the page draws.
  "documents.menu.approvals": { mn: "Батлах дараалал", en: "Approval queue" },

  "documents.field.title": { mn: "Баримтын гарчиг", en: "Document Title" },
  "documents.field.title_placeholder": { mn: "e.g. Хамтран ажиллах гэрээ 2026", en: "e.g. Partnership agreement 2026" },
  "documents.field.category": { mn: "Баримтын ангилал", en: "Document Category" },
  "documents.field.signature": { mn: "Тоон гарын үсэг (E-ID / ДАН)", en: "Digital Signature (E-ID / DAN)" },
  "documents.field.created": { mn: "Үүсгэсэн огноо", en: "Created Date" },
  "documents.field.signature_method": { mn: "Гарын үсгийн арга", en: "Signature Method" },
  "documents.field.reg_number": { mn: "Регистрийн дугаар", en: "Registration Number" },
  "documents.field.otp_code": { mn: "Нэг удаагийн нууц код (OTP)", en: "One-Time Code (OTP)" },
  "documents.field.verification_code": { mn: "Баталгаажуулах код", en: "Verification code" },
  "documents.field.signed_at": { mn: "Зурсан цаг", en: "Signed at" },
  "documents.field.certificate_serial": { mn: "Гэрчилгээний дугаар", en: "Certificate serial" },
  "documents.field.certificate_issuer": { mn: "Гэрчилгээ олгогч", en: "Certificate issuer" },
  "documents.field.approval_reference": { mn: "Батламжийн сурвалж", en: "Approval reference" },
  "documents.field.waiting_days": { mn: "Хүлээсэн хоног", en: "Days waiting" },

  "documents.stat.awaiting": { mn: "Гарын үсэг хүлээж буй", en: "Awaiting signature" },
  "documents.message.showing_some": {
    mn: "{total} баримтаас хамгийн шинэ {shown}-г харуулж байна.",
    en: "Showing the {shown} most recent of {total} documents.",
  },
  "documents.message.showing_some_oldest": {
    mn: "Хүлээж байгаа {total}-аас хамгийн урт хүлээсэн {shown}-г харуулж байна.",
    en: "Showing the {shown} longest-waiting of {total}.",
  },
  "documents.action.load_more": { mn: "Дараагийнхыг ачаалах", en: "Load more" },
  "documents.message.stale_rows": {
    mn: "Эдгээр мөр хоцрогдсон байж болно — жагсаалтыг шинэчилж чадсангүй.",
    en: "These rows may be out of date — the list could not be refreshed.",
  },
  "documents.action.retry": { mn: "Дахин оролдох", en: "Try again" },
  "documents.field.search_placeholder": { mn: "Гарчгаар хайх", en: "Search by title" },
  "documents.field.any_type": { mn: "Бүх төрөл", en: "Any type" },
  "documents.field.any_status": { mn: "Бүх төлөв", en: "Any status" },
  "documents.message.no_matches": {
    mn: "Хайлтад тохирох баримт олдсонгүй. Хайлт эсвэл шүүлтийг сольж үзээрэй.",
    en: "No documents match. Try a different search or filter.",
  },
  "documents.stat.oldest_days": { mn: "Хамгийн урт хүлээлт (хоног)", en: "Longest wait (days)" },

  "documents.category.legal_contract": { mn: "Гэрээ", en: "Legal Contract" },
  "documents.category.official_request": { mn: "Албан хүсэлт", en: "Official Request" },
  "documents.category.internal_approval": { mn: "Дотоод батламж", en: "Internal Approval" },

  "documents.state.pending_signature": { mn: "Гарын үсэг хүлээж буй", en: "Pending Signature" },
  "documents.state.draft": { mn: "Ноорог", en: "Draft" },
  "documents.state.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "documents.state.approved": { mn: "Баталсан", en: "Approved" },
  "documents.state.rejected": { mn: "Татгалзсан", en: "Rejected" },
  "documents.state.awaiting_now": { mn: "Одоо хүлээж байна", en: "Awaiting now" },
  "documents.state.awaiting_later": { mn: "Дараа", en: "Later" },
  "documents.state.never_given": { mn: "Аваагүй", en: "Never given" },

  "documents.action.create": { mn: "Баримт үүсгэх", en: "Create Document" },
  "documents.action.sign": { mn: "Гарын үсэг зурах", en: "Sign" },
  "documents.action.reject": { mn: "Татгалзах", en: "Reject" },
  "documents.action.request_approval": { mn: "Батлах хүсэлт илгээх", en: "Send approval request" },
  "documents.action.view_history": { mn: "Гарын үсгийн түүх", en: "Signature history" },
  "documents.action.route": { mn: "Батлахад илгээх", en: "Send for approval" },

  "documents.message.loading": { mn: "Баримтуудыг ачаалж байна...", en: "Loading documents..." },
  "documents.message.empty": {
    mn: "Одоогоор баримт байхгүй. Баримт үүсгээд E-ID / ДАН гарын үсэгт илгээнэ үү.",
    en: "No documents yet. Create one and route it for an E-ID / DAN signature.",
  },
  "documents.message.signing": { mn: "Гарын үсэг зурж байна...", en: "Signing..." },
  "documents.message.no_pending": {
    mn: "Батлах хүлээж байгаа баримт байхгүй.",
    en: "Nothing is waiting for approval.",
  },
  "documents.message.sign_not_granted": {
    mn: "Танд гарын үсэг зурах эрх (documents.sign) байхгүй тул үйлдлүүд харагдахгүй.",
    en: "The actions are hidden because you do not hold the documents.sign permission.",
  },
  "documents.message.rejected_not_signed": {
    mn: "Татгалзсан — гарын үсэг зураагүй",
    en: "Rejected — not signed",
  },
  "documents.message.sign_success": {
    mn: "\"{title}\"-д {method}-ээр гарын үсэг зурлаа.",
    en: "\"{title}\" was successfully signed via {method}.",
  },
  "documents.message.sign_failed": { mn: "Гарын үсэг зурж чадсангүй", en: "Signature failed" },
  "documents.message.history_failed": {
    mn: "Гарын үсгийн түүхийг ачаалж чадсангүй",
    en: "Could not load the signature history",
  },
  "documents.message.signature_outside_chain": {
    mn: "хэлхээнээс гадуур",
    en: "outside the chain",
  },
  "documents.message.step_open_to_anyone": {
    mn: "хэн ч зурж болно",
    en: "open to any signer",
  },
  "documents.message.no_signatures": {
    mn: "Энэ баримтад гарын үсэг зураагүй байна.",
    en: "Nothing has been signed on this document yet.",
  },

  // The E-ID ceremony: the citizen approves on their own device, so the screen
  // explains what is happening on the other end of it.
  "documents.message.eid_method_hint": {
    mn: "Иргэний eID апп руу батлах хүсэлт илгээгдэнэ. Тэр өөрийн төхөөрөмж дээрээ баримтын нэрийг харж, өөрийн гэрчилгээгээр батална.",
    en: "An approval request is pushed to the citizen's eID app. They see the document's name on their own device and approve it with their own certificate.",
  },
  "documents.message.dan_method_hint": {
    mn: "ДАН нь батлах хүсэлт түлхдэггүй тул регистр ба нэг удаагийн код хэрэгтэй.",
    en: "DAN pushes no approval request, so it needs a registration number and a one-time code.",
  },
  "documents.message.verification_code_hint": {
    mn: "Иргэний төхөөрөмж дээр яг ижил код харагдах ёстой. Зөрвөл батлахгүй.",
    en: "The citizen's device must show exactly this code. If it differs, they should not approve.",
  },
  "documents.message.awaiting_approval": {
    mn: "{reg}-ийн батламжийг хүлээж байна...",
    en: "Waiting for {reg} to approve...",
  },
  "documents.message.approval_display_text": {
    mn: "Иргэнд харагдах текст",
    en: "What the citizen sees",
  },
  "documents.message.approval_refused": {
    mn: "Иргэн батлахаас татгалзсан — гарын үсэг зурагдаагүй.",
    en: "The citizen declined the request — nothing was signed.",
  },
  "documents.message.approval_expired": {
    mn: "Батлах хүсэлтийн хугацаа дууссан — гарын үсэг зурагдаагүй. Дахин илгээнэ үү.",
    en: "The approval request expired — nothing was signed. Send it again.",
  },
  "documents.message.reject_confirm": {
    mn: "\"{title}\"-г татгалзах уу? Үүнийг буцаах боломжгүй.",
    en: "Reject \"{title}\"? This cannot be undone.",
  },
  "documents.message.reject_success": { mn: "\"{title}\"-г татгалзлаа.", en: "\"{title}\" was rejected." },
  "documents.message.reject_failed": { mn: "Татгалзаж чадсангүй", en: "Reject failed" },
  "documents.message.create_failed": { mn: "Баримт үүсгэж чадсангүй", en: "Failed to create document" },
  "documents.message.create_success": {
    mn: "\"{title}\" үүсгэж, батлах дараалалд оруулав.",
    en: "\"{title}\" was created and sent for approval.",
  },
  "documents.message.route_success": {
    mn: "\"{title}\" батлах дараалалд орлоо.",
    en: "\"{title}\" was sent for approval.",
  },
  "documents.message.route_failed": { mn: "Батлахад илгээж чадсангүй", en: "Could not send it for approval" },
  "documents.message.load_failed": {
    mn: "Баримтуудыг ачаалж чадсангүй. Дахин оролдоно уу.",
    en: "The documents could not be loaded. Try again.",
  },
  "documents.message.step_needs_name": {
    mn: "Шат бүрд нэр шаардлагатай.",
    en: "Every step needs a name.",
  },
  "documents.message.signature_progress": {
    mn: "{required} гарын үсгээс {applied} зурагдсан",
    en: "{applied} of {required} signatures applied",
  },

  // Templates
  "documents.menu.templates": { mn: "Баримтын загвар", en: "Document templates" },
  "documents.view.templates_hint": {
    mn: "Баримт үүсгэхэд хэрэглэх бэлдэц: гарчгийн загвар ба ангилал.",
    en: "Presets a document is started from: a title pattern and a category.",
  },
  "documents.field.template_name": { mn: "Загварын нэр", en: "Template name" },
  "documents.field.title_pattern": { mn: "Гарчгийн загвар", en: "Title pattern" },
  "documents.action.add_template": { mn: "Загвар нэмэх", en: "Add template" },
  "documents.action.use_template": { mn: "Баримт үүсгэх", en: "Create document" },
  // The braces here are literal: this text teaches the syntax, so it is called
  // without vars and t() leaves them alone. Do not "fix" them into placeholders.
  "documents.message.title_pattern_hint": {
    mn: "Гарчигт {year}, {month}, {date} гэж бичвэл загварыг хэрэглэх үед орлуулагдана. Танихгүй хэсэг хөндөгдөхгүй.",
    en: "A title may hold {year}, {month} or {date}; they are filled in when the template is used. Anything else is left as written.",
  },
  "documents.message.no_templates": {
    mn: "Загвар байхгүй. Дээрээс нэгийг нэмнэ үү.",
    en: "No templates yet. Add one above.",
  },
  "documents.message.templates_failed": { mn: "Загваруудыг ачаалж чадсангүй", en: "Could not load templates" },
  "documents.message.template_saved": { mn: "Загвар хадгалагдлаа.", en: "Template saved." },
  "documents.message.template_active_hint": {
    mn: "Идэвхгүй загвараас баримт үүсгэхгүй, гэхдээ бүртгэл хадгалагдана.",
    en: "An inactive template produces no documents, but its record is kept.",
  },
  "documents.message.template_unsaved": {
    mn: "Хадгалаагүй засвар байна — эхлээд хадгална уу.",
    en: "There are unsaved changes — save them first.",
  },
  "documents.message.template_inactive": {
    mn: "Идэвхгүй загвар — эхлээд идэвхжүүлнэ үү.",
    en: "This template is inactive — activate it first.",
  },
  "documents.message.template_used": {
    mn: "\"{title}\" баримт үүсч, батлах дараалалд орлоо.",
    en: "\"{title}\" was created and is waiting for approval.",
  },
  "documents.message.template_delete_confirm": {
    mn: "\"{name}\" загварыг устгах уу?",
    en: "Delete the template \"{name}\"?",
  },
  "documents.message.template_deleted": { mn: "\"{name}\" загварыг устгалаа.", en: "Template \"{name}\" was deleted." },

  // Signature policies
  "documents.menu.signature_policies": { mn: "Гарын үсгийн бодлого", en: "Signature policies" },
  "documents.view.signature_policies_hint": {
    mn: "Баримтын төрөл тус бүрийг ямар сувгаар гарын үсэг зурж болохыг тогтооно.",
    en: "Which national channel may sign each document type, and who is allowed to.",
  },
  "documents.field.require_named_signer": { mn: "Зөвхөн нэрлэсэн хүн", en: "Named signer only" },
  "documents.state.policy_configured": { mn: "Тохируулсан", en: "Configured" },
  "documents.state.policy_default": { mn: "Үндсэн", en: "Default" },
  "documents.message.policies_failed": { mn: "Бодлогуудыг ачаалж чадсангүй", en: "Could not load policies" },
  "documents.message.policy_saved": { mn: "{type}-ийн бодлого хадгалагдлаа.", en: "The {type} policy was saved." },
  "documents.message.policy_named_signer_hint": {
    mn: "\"Зөвхөн нэрлэсэн хүн\" гэдэг нь баримтын урсгалд регистрийн дугаараар нэрлэгдсэн хүн л гарын үсэг зурна гэсэн үг. Урсгалд хэн ч нэрлэгдээгүй бол энэ тохиргоо хадгалагдахгүй — тэр төрөл гарын үсэг зурах боломжгүй болох тул.",
    en: "\"Named signer only\" means the signature must come from a registration number the type's approval chain names. It cannot be saved while the chain names nobody, because that would leave the type unsignable.",
  },

  // Approval chains
  "documents.menu.workflows": { mn: "Баримтын урсгал", en: "Document workflows" },
  "documents.view.workflows_hint": {
    mn: "Баримтын төрөл тус бүр хэдэн хүний гарын үсгийг ямар дараалалтай шаардахыг тогтооно.",
    en: "How many signatures each document type needs, and in what order.",
  },
  "documents.action.add_step": { mn: "Шат нэмэх", en: "Add step" },
  "documents.field.step_name": { mn: "Шатны нэр", en: "Step name" },
  "documents.field.step_signer": { mn: "Гарын үсэг зурагчийн регистр", en: "Signer's registration number" },
  "documents.message.workflows_failed": { mn: "Урсгалуудыг ачаалж чадсангүй", en: "Could not load approval chains" },
  "documents.message.workflow_saved": { mn: "{type}-ийн урсгал хадгалагдлаа.", en: "The {type} approval chain was saved." },
  "documents.message.chain_single_signature": {
    mn: "Шат тохируулаагүй — нэг гарын үсэг батална",
    en: "No steps — a single signature approves",
  },
  "documents.message.chain_signature_count": {
    mn: "{count} гарын үсэг шаардана",
    en: "{count} signatures required",
  },
  "documents.message.no_steps": {
    mn: "Шат байхгүй. Ингэснээр нэг гарын үсэг баримтыг батална.",
    en: "No steps. One signature approves the document.",
  },
  "documents.message.step_signer_hint": {
    mn: "Регистрийн дугаарыг хоосон орхивол шат тоологдох ч тодорхой хүн нэрлэгдэхгүй — гарын үсэг зурах эрхтэй хэн ч зурж болно.",
    en: "Leave a registration number empty and the step still counts, but names nobody: anyone who may sign can take it.",
  },

  // Retention
  "documents.menu.retention": { mn: "Хадгалалтын дүрэм", en: "Retention rules" },
  "documents.view.retention_hint": {
    mn: "Баримтын төрөл тус бүрийг хэдэн жил хадгалахыг тогтооно.",
    en: "How many years a document of each type is kept.",
  },
  "documents.field.retain_years": { mn: "Хадгалах жил", en: "Retain (years)" },
  "documents.field.retention_note": { mn: "Тайлбар", en: "Note" },
  "documents.stat.filed": { mn: "Бүртгэсэн баримт", en: "Documents filed" },
  "documents.stat.past_term": { mn: "Хугацаа дууссан", en: "Past their term" },
  "documents.stat.rules_set": { mn: "Тохируулсан дүрэм", en: "Rules set" },
  "documents.message.retention_failed": { mn: "Дүрмүүдийг ачаалж чадсангүй", en: "Could not load retention rules" },
  "documents.message.retention_saved": { mn: "{type}-ийн дүрэм хадгалагдлаа.", en: "The {type} retention rule was saved." },
  "documents.message.retention_no_deletion": {
    mn: "Энэ хуваарь ямар ч баримтыг устгахгүй. Хугацаа дууссаныг л харуулна — шийдвэрийг хүн гаргана.",
    en: "Nothing is deleted on this schedule. The screen only reports what is past its term; a person decides.",
  },
} as const;
