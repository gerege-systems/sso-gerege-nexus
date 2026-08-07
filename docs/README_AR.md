<div dir="rtl">

# Gerege Nexus

**منصة متكاملة للعمليات الرقمية**

**Gerege Nexus** منصة معيارية مفتوحة المصدر تربط الخدمات والعمليات والأنظمة
والبيانات عبر المؤسسات العامة والخاصة. تضع اللغة **المنغولية في المقام الأول**،
وتتكامل مباشرة مع البنية التحتية الرقمية الوطنية في منغوليا (DAN و E-ID و
XYP / ХУР).

كلمة *Nexus* تعني نقطة الاتصال: حيث تلتقي المؤسسات والخدمات وسير العمل والأنظمة
والمستخدمون والبيانات. المنصة نفسها ليست مرتبطة بقطاع واحد — الوحدات التي تعمل
فوقها هي ما يحدد طبيعة كل عملية نشر.

تُجمَّع الوحدات في ملف تنفيذي واحد بلغة Go، بينما يقرر متجر تطبيقات مدعوم بـ
PostgreSQL أي التطبيقات مفعَّلة لكل مستأجر — فصل معياري دون قفزات الشبكة أو
الكلفة التشغيلية للخدمات المصغَّرة.

</div>

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> <b>العربية</b>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <a href="README_FR.md"><img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> Français</a>
  &nbsp;·&nbsp;
  <a href="README_RU.md"><img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> Русский</a>
  &nbsp;·&nbsp;
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-15.1-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

<div dir="rtl">

## المحتويات

- [المؤلفون](#المؤلفون)
- [القدرات الأساسية](#القدرات-الأساسية)
- [تطبيقات الأعمال](#تطبيقات-الأعمال)
- [بنية المستودع](#بنية-المستودع)
- [البدء](#البدء)
- [الإعدادات](#الإعدادات)
- [نظرة عامة على الواجهة البرمجية](#نظرة-عامة-على-الواجهة-البرمجية)
- [الاختبارات وضوابط الجودة](#الاختبارات-وضوابط-الجودة)
- [الأمان](#الأمان)
- [فهرس التوثيق](#فهرس-التوثيق)

---

## المؤلفون

</div>

| المساهم | الدور |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | البنية المعمارية، نواة المنصة |
| **Gemini AI** | توليد الشيفرة، التوثيق |
| **Claude AI** | تحليل الشيفرة، تدقيق الأمان |

---

<div dir="rtl">

## القدرات الأساسية

### ١. نواة معيارية أحادية عالية الأداء

- **وحدات Go مُجمَّعة وقت البناء** — `contacts` و `products` و `inventory` و
  `billing` و `documents` و `developer_portal` تُجمَّع في ملف تنفيذي واحد
  وتُستدعى داخل العملية نفسها.
- **متجر تطبيقات لكل مستأجر** — صلاحيات التطبيقات والقوائم و RBAC تُدار من
  PostgreSQL (`app_installations`).
- **محلِّل التبعيات** — حل تكراري على رسم بياني موجَّه لا دوري، مع كشف الدورات
  والتحقق من قيود semver.
- **مزامنة الفهرس** — `catalog/apps.json` هو المصدر الوحيد للحقيقة، ويُوفَّق
  جدول `apps` معه عند كل إقلاع.

### ٢. محرك مرونة سحابي المنشأ

</div>

| الوحدة | الغرض |
| --- | --- |
| `resilience/breaker.go` | قاطع دائرة تكيُّفي على نهج SRE في Google |
| `resilience/loadshedder.go` | إسقاط الحمل بـ `503` + `Retry-After` عند الضغط |
| `resilience/singleflight.go` | دمج العمل المكرَّر الجاري تنفيذه |
| `resilience/retry.go` | إعادة المحاولة بتراجع أُسّي |

<div dir="rtl">

### ٣. البنية التحتية الرقمية الوطنية

- **XYP — تبادل معلومات الدولة** (`platform/gerege/xyp.go`): السجل المدني
  للمواطنين (`WS100101`) والتحقق من الكيانات الاعتبارية (`WS100201`).
- **الهوية الرقمية الوطنية E-ID و DAN**
  ([`developer.gerege.mn`](https://developer.gerege.mn)،
  [`eidmongolia.mn`](https://eidmongolia.mn)) — توقيع رقمي بالبنية التحتية
  للمفاتيح العامة، ورمز لمرة واحدة عبر الهاتف، ودخول موحَّد مصرفي، وتحقق
  بيومتري من الوجه.
- **مزوِّد OAuth2 / OIDC مدمج** (`/.well-known/openid-configuration`) يُصدر
  رموز client-credentials للأنظمة الخارجية.

> **ملاحظة.** وضع المحاكاة لـ E-ID و DAN و XYP هو تسهيل للتطوير فقط. مع
> `ENVIRONMENT=production` يُعطَّل تلقائيًا، فلا يمكن مطلقًا لرقم تسجيل مُلفَّق
> أن يجتاز المصادقة.

### ٤. مساعد الذكاء الاصطناعي والتحليلات

- **المساعد الذكي** (`platform/ai/copilot.go`) — محادثة مُصنَّفة حسب النية
  وموصولة ببيانات المستأجر الحية.
- **متنبئ الطلب على المخزون** (`platform/ai/inventory_forecaster.go`) — توصيات
  لمخزون الأمان ونقطة إعادة الطلب انطلاقًا من حركة المخزون التاريخية.

---

## تطبيقات الأعمال

</div>

| # | التطبيق | المعرِّف | المسار | الوصف |
| --- | --- | --- | --- | --- |
| ١ | جهات الاتصال | `io.example.contacts` | `/contacts` | دليل العملاء والمورِّدين مع تعبئة تلقائية من XYP |
| ٢ | المنتجات | `io.example.products` | `/products` | الفهرس والتسعير ووحدات SKU لكل مستأجر |
| ٣ | المخزون | `io.example.inventory` | `/inventory` | المستودعات ومستويات المخزون وسجل الحركة |
| ٤ | الفوترة و e-Barimt | `io.example.billing` | `/billing` | إصدار الفواتير، ضريبة ١٠٪، إيصالات e-Barimt |
| ٥ | المستندات الرقمية والتوقيع الإلكتروني | `io.example.documents` | `/documents` | توجيه المستندات والتوقيعات والموافقات |
| ٦ | بوابة المطوِّرين و SSO عبر OAuth2 | `io.example.developer_portal` | `/developer/apps` | تسجيل عملاء OAuth2 |

<div dir="rtl">

لا تُفتح المسارات إلا بعد تثبيت التطبيق وتفعيله للمستأجر؛ وإلا فإن البوابة
تُعيد `403 Forbidden`.

---

## بنية المستودع

</div>

```
backend/
  cmd/api/            خادم واجهة HTTP البرمجية (+ بيانات العرض التجريبي)
  cmd/migrate/        مُنفِّذ ترحيلات Goose
  db/migrations/      ترحيلات SQL
  internal/
    module.go         عقد وحدة Go
    apps/             وحدات الأعمال
    platform/         خدمات نواة المنصة
frontend/             عميل الويب Next.js 15 (App Router)
catalog/              فهرس متجر التطبيقات وبياناته الوصفية
deploy/               Dockerfile الإنتاج وإعدادات Nginx
docs/                 التوثيق والترجمات
```

---

<div dir="rtl">

## البدء

### المتطلبات المسبقة

- Go 1.25+
- Node.js 20+
- PostgreSQL 16+ (أو Docker Compose)

### ١. Docker Compose

</div>

```bash
docker compose up -d
```

<div dir="rtl">

تعمل الترحيلات في خدمة `migrate` مخصَّصة تعمل لمرة واحدة قبل بدء الواجهة
البرمجية.

### ٢. يدويًا

**الواجهة الخلفية:**

</div>

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

<div dir="rtl">

**الواجهة الأمامية:**

</div>

```bash
cd frontend
npm ci
npm run dev
```

<div dir="rtl">

افتح [http://localhost:3000](http://localhost:3000).

### بيانات الدخول التجريبية

</div>

| الحقل | القيمة |
| --- | --- |
| البريد الإلكتروني | `admin@example.com` |
| كلمة المرور | `Password123!` |
| المستأجر | `Demo Corporation` (`slug: demo`) |

<div dir="rtl">

يُنشأ حساب العرض التجريبي خارج بيئة الإنتاج فقط. أما في الإنتاج فلا يُنشأ إلا
عند ضبط `SEED_DEMO_DATA=true` صراحةً.

---

## النشر الآلي

نجاح [`ci.yml`](../.github/workflows/ci.yml) على `main` هو ما يُشغِّل
[`deploy.yml`](../.github/workflows/deploy.yml) — لا الدفع نفسه، فلا يُنشَر أبدًا
إيداع فشلت اختباراته:

١. بناء صور الواجهة الخلفية والأمامية ورفعها إلى GHCR (`:latest` و `:<sha>`).

٢. نسخ `docker-compose.prod.yml` إلى الخادم.

٣. كتابة ملف `.env` على الخادم من أسرار GitHub وسحب الصور.

٤. تشغيل الترحيلات حتى اكتمالها، ثم تبديل الواجهة البرمجية والواجهة الأمامية.

٥. فحص `/health` و `/ready`، وطباعة سجلات الحاويات وإفشال التشغيل إذا لم يكن
النشر سليمًا.

للنشر يدويًا: Actions ← *Deploy to Production* ← **Run workflow**، مع إمكانية
تثبيت وسم صورة محدَّد.

الأسرار المطلوبة في المستودع:

</div>

| السر | مطلوب | الوصف |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | نعم | المفتاح الخاص لمستخدم النشر. بدونه يُتخطَّى النشر |
| `POSTGRES_PASSWORD` | نعم | كلمة مرور قاعدة البيانات على الخادم |
| `SSO_DEFAULT_CLIENT_SECRET` | نعم | إلزامي لعميل OAuth2 المدمج في الإنتاج |
| `DEPLOY_HOST` | **نعم** | لا قيمة افتراضية |
| `PUBLIC_ORIGIN` (متغيِّر مستودع) | **نعم** | لا قيمة افتراضية |
| `DEPLOY_USER` / `DEPLOY_PORT` | لا | الافتراضي `deploy` / `22` |

<div dir="rtl">

> هذا المستودع نسخة متفرِّعة من `open-gerege-nexus` و**يتشارك معه الخادم
> نفسه**: هذا يستجيب على `sso.gerege.mn` وجاره على `nexus.gerege.mn`. لذلك لا
> توجد قيمة افتراضية لـ `DEPLOY_HOST` ولا لـ `PUBLIC_ORIGIN` — فالسر المنسي
> يوقف سير العمل بدل أن ينشر هذا المستودع فوق إنتاج جاره. يحدِّد
> `PUBLIC_ORIGIN` في موضع واحد سياسة CORS ومُصدِر OIDC وعنوان استدعاء eID، لذا
> فإن تغييره يستتبع معه DNS وشهادة TLS وكل عميل ثبَّت المُصدِر لديه.

لا يحتاج الخادم سوى Docker — دون شيفرة مصدرية ودون أدوات Go/Node. راجع
[`deploy/.env.prod.example`](../deploy/.env.prod.example) للاطلاع على القيم.

---

## الإعدادات

راجع [`.env.example`](../.env.example) للقائمة الكاملة.

</div>

| المتغيِّر | الافتراضي | الوصف |
| --- | --- | --- |
| `DATABASE_URL` | localhost | سلسلة الاتصال بـ PostgreSQL |
| `PORT` | `8080` | منفذ استماع الواجهة البرمجية |
| `ENVIRONMENT` | `development` | القيمة `production` تُفعِّل الإعدادات المُشدَّدة |
| `APP_CATALOG_PATH` | `catalog/apps.json` | مسار فهرس متجر التطبيقات |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | قائمة المصادر المسموح بها (CORS) |
| `TRUST_PROXY_HEADERS` | `false` | هل يُوثَق بترويسة `X-Forwarded-For` |
| `SEED_DEMO_DATA` | مُفعَّل خارج الإنتاج | إنشاء حساب العرض التجريبي |
| `SSO_DEFAULT_CLIENT_SECRET` | — | مطلوب في الإنتاج |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | مُفعَّل خارج الإنتاج | محاكاة التكاملات الوطنية |

---

<div dir="rtl">

## نظرة عامة على الواجهة البرمجية

</div>

| الطريقة | المسار | الوصف |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | فحوص الحياة والجاهزية |
| `GET` | `/metrics` | مقاييس Prometheus |
| `POST` | `/api/v1/auth/login` | تسجيل الدخول بالبريد وكلمة المرور |
| `POST` | `/api/v1/auth/eid/login` | تسجيل الدخول بالهوية الرقمية الوطنية |
| `POST` | `/api/v1/auth/dan/login` | تسجيل الدخول عبر بوابة DAN |
| `POST` | `/api/v1/auth/logout` | إبطال الجلسة |
| `GET` | `/api/v1/menus` | قوائم التطبيقات المفعَّلة للمستأجر |
| `GET` | `/api/v1/store/apps` | قائمة متجر التطبيقات |
| `POST` | `/api/v1/store/apps/{slug}/install` | تثبيت تطبيق (مسؤول) |
| `POST` | `/oauth2/token` | رمز OAuth2 من نوع client credentials |

<div dir="rtl">

تنتقل رموز الجلسة إما في ملف تعريف ارتباط HttpOnly أو عبر
`Authorization: Bearer <token>`.

---

## الاختبارات وضوابط الجودة

</div>

```bash
# اختبارات وحدات الواجهة الخلفية مع كاشف التسابق
cd backend && go test -race ./...

# التحليل الساكن
cd backend && go vet ./... && golangci-lint run

# فحص الثغرات
cd backend && govulncheck ./...

# بناء الواجهة الأمامية
cd frontend && npm run build
```

<div dir="rtl">

تُشغِّل منظومة التكامل المستمر الفحص اللغوي والاختبارات وبناء الواجهة الأمامية
وبناء صورة Docker و govulncheck و gosec عند كل دفع وكل طلب دمج.

---

## الأمان

- رموز الجلسة قيم عشوائية بطول ٢٥٦ بت، ولا يُخزَّن منها سوى بصمة SHA-256.
- تُجزَّأ كلمات المرور باستخدام bcrypt، ومحاولات تسجيل الدخول محدودة المعدل لكل
  عنوان IP.
- يتطلب تثبيت التطبيقات أو تفعيلها أو تعطيلها وتسجيل التكاملات صلاحيات مسؤول
  المستأجر.
- تستخدم مصادقة عملاء OAuth2 مقارنة ذات زمن ثابت.

أبلغ عن الثغرات وفق ما هو موضَّح في [`SECURITY.md`](../SECURITY.md).

---

## فهرس التوثيق

</div>

| المستند | الوصف |
| --- | --- |
| [مركز التوثيق](README.md) | فهرس كل المستندات والترجمات |
| [مواصفات البنية المعمارية](ARCHITECTURE_SPECIFICATION.md) | طبقات المنصة وقرارات التصميم |
| [دليل تأليف الوحدات](MODULE_AUTHORING_GUIDE.md) | كيفية بناء وحدة تطبيق جديدة |
| [المساهمة](../CONTRIBUTING.md) | سير عمل المساهمة |
| [سياسة الأمان](../SECURITY.md) | الإبلاغ عن الثغرات |
| [مدونة السلوك](../CODE_OF_CONDUCT.md) | معايير المجتمع |
| [سجل التغييرات](../CHANGELOG.md) | تاريخ الإصدارات |

---

<div dir="rtl">

## الشكر ومصادر الإلهام

١. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
من **[@snykk](https://github.com/snykk)** — أسس واجهة REST البرمجية بلغة Go.

٢. **[Odoo](https://github.com/odoo/odoo)** — متجر التطبيقات المعياري ونموذج
التبعيات.

٣. **[go-zero](https://github.com/zeromicro/go-zero)** — محرك المرونة السحابي
المنشأ.

---

## الترخيص

حقوق النشر (c) 2026 **Gerege Systems Development Team, Gemini AI &
Claude AI**. يُوزَّع بموجب رخصة Apache 2.0 — راجع [`LICENSE`](../LICENSE).

أيقونات الأعلام من [Flaticon](https://www.flaticon.com/)
([بيان الإسناد](assets/icons/ATTRIBUTION.md)).

</div>
