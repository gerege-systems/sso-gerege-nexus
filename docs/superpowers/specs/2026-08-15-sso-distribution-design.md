# Gerege SSO — Түвшин 2 distribution болгон дахин барих

**Огноо:** 2026-08-15
**Repo:** `github.com/gerege-systems/sso-gerege-nexus`
**Цөм:** `github.com/gerege-systems/open-gerege-nexus/backend v1.6.0`

---

## 1. Яагаад

`sso-gerege-nexus` өнөөдөр цөмийн **hard fork** — upstream-тэй харьцуулахад 698
файлын зөрүү, 87,882 мөр устсан, 11,688 мөр нэмэгдсэн. Цөмийн бүх код (backend,
frontend, docs, native-apps) энд хуулагдсан байна.

Экосистемийн git стратеги ([ECOSYSTEM_GIT_STRATEGY.md][strategy]) үүнийг
шууд хориглодог: *"Nexus-ийн кодыг хэн ч хуулахгүй, зөвхөн dependency болгон
ашиглана."* Зуун fork = засвар бүр зуун газар давтагдана.

`appstore-gerege-nexus` аль хэдийн зөв хэлбэрт шилжсэн — энэ баримт бичиг
`sso-gerege-nexus`-ийг яг тэр хэлбэрт оруулах ажлыг тодорхойлно.

[strategy]: https://github.com/gerege-systems/open-gerege-nexus/blob/main/docs/ECOSYSTEM_GIT_STRATEGY.md

## 2. Хамрах хүрээ

**Хийх:** одоо байгаа бүх файлыг устгаж, orphan branch дээр Түвшин 2
distribution-ыг шинээр барих. Дөрвөн SSO модуль, өөрийн схем, каталог, CI,
deploy.

**Хийхгүй:** цөмийн код өөрчлөх (§7-д нэрлэсэн хоёр upstream PR-аас бусад),
frontend бичих, хуучин fork-ийн бизнес модулиудыг (`gov_services`, `billing`,
`inventory`, `products`, `developer_portal`) авч үлдэх.

**Устгагдах кодын хувь заяа:** `gov_services` (≈2,000 мөр, тесттэй) болон
бусад бизнес модулиуд GitHub дээрх хуучин `main`-ий түүхэнд үлдэнэ. Orphan
branch force push хийхэд тэдгээр нь **GitHub-аас алга болно**. Хэрэв тэднийг
хожим сэргээх бодолтой бол §10-ын эхний алхмыг хийнэ.

## 3. Repo-ийн зорилтот бүтэц

```
sso-gerege-nexus/
  go.mod                  цөмийг tag-аар авна + tool cmd/migrate
  go.sum
  main.go                 platform.Run — дөрвөн модуль бүртгэнэ, өөр юу ч биш
  catalog_test.go         каталог ↔ компилдсэн модулиуд тэнцүү эсэх
  route_policy_test.go    нээлттэй зам байхгүйг барина

  modules/
    federation/           итгэсэн гадны IdP-ууд
    sessions/             идэвхтэй сесс, төхөөрөмж
    accessreview/         эрхийн дахин баталгаажуулалт
    provisioning/         SCIM 2.0 гадагш түлхэлт

  catalog/
    apps.json             энэ бүтээгдэхүүний 4 апп + цөмийн хуулбар бичлэгүүд
    manifests/*.json
    chronicle/*.json

  db/migrations/
    00001_sso_schema.sql  энэ repo-гийн эзэмшдэг цорын ганц түүх

  deploy/
    Dockerfile
    docker-compose.yml
    nginx/sso.gerege.mn.conf

  docs/superpowers/specs/2026-08-15-sso-distribution-design.md   (энэ файл)

  .github/workflows/{ci,security,deploy}.yml
  .github/dependabot.yml
  README.md  LICENSE  .gitignore  .dockerignore
```

Frontend хавтас **байхгүй**. Цөмийн shell каталогоор жолоодогддог тул шинэ
модулиудын цэсний бичлэгүүд өөрөө гарна; хуудас нь `[app]/[feature]`-ийн
"coming soon" рендер (§7).

## 4. `main.go`

```go
func main() {
	err := platform.Run(platform.Options{
		Modules: func(p nexus.Platform) {
			federation.New(p)
			sessions.New(p)
			accessreview.New(p)
			provisioning.New(p)
		},
	})
	if err != nil {
		slog.Error("Gerege SSO зогслоо", "error", err)
		os.Exit(1)
	}
}
```

Бизнес логик энд байхгүй. Модулиуд хоорондоо хамааралгүй тул бүртгэлийн
дараалал утгагүй — цагаан толгойн дарааллаар.

## 5. Дөрвөн модуль

Тус бүр `nexus.Module` (7 метод) ба `nexus.AccessPolicy` (2 метод)
хэрэгжүүлнэ. Бүх зам `RoutePermissionPrefix()`-ээр хаалганд ордог: GET/HEAD →
`<prefix>.read`, бусад → `<prefix>.manage`.

### 5.1 `federation` — `io.gerege.nexus.sso_federation`

Энэ суулгац ямар гадны IdP-д итгэхийг удирдана.

| | |
|---|---|
| Permission prefix | `sso_federation` |
| Цэс | `/module/sso-federation`, icon `network`, order 10 |
| Хүснэгт | `sso_federation_providers`, `sso_federation_links` |

`sso_federation_providers` — тенант бүрийн итгэсэн IdP: `id`, `tenant_id`,
`display_name`, `issuer`, `client_id`, `client_secret_encrypted`, `scopes`,
`attribute_map` (JSONB), `enabled`, `created_at/by`. `UNIQUE (tenant_id, issuer)`.

`sso_federation_links` — тухайн IdP дээрх хэн энд хэн болох нь: `provider_id`,
`subject`, `user_id`, `linked_at`. `UNIQUE (provider_id, subject)`.

Client secret нь **шифрлэгдэж** хадгалагдана (цөмийн `platform/security`-ийн
арга; хэрэв экспортлогдоогүй бол AES-GCM + `SSO_FEDERATION_KEY` env). Уншиж
буцаах API байхгүй — зөвхөн бичих ба орлуулах.

API `/api/v1/sso/federation`:

| Метод | Зам | Юу хийх |
|---|---|---|
| `GET` | `/providers` | Жагсаалт (secret буцаахгүй) |
| `POST` | `/providers` | Шинэ IdP бүртгэх |
| `PUT` | `/providers/{id}` | Засах |
| `PUT` | `/providers/{id}/enabled` | Асаах / унтраах |
| `DELETE` | `/providers/{id}` | Устгах (link-үүд cascade) |
| `GET` | `/providers/{id}/links` | Тухайн IdP-ээр холбогдсон хүмүүс |

### 5.2 `sessions` — `io.gerege.nexus.sso_sessions`

Холбогдсон бүх апп дээрх идэвхтэй сессүүдийг харах, алсаас таслах.

| | |
|---|---|
| Permission prefix | `sso_sessions` |
| Цэс | `/module/sso-sessions`, icon `monitor-smartphone`, order 20 |
| Хүснэгт | `sso_session_events` |

`sso_session_events` — энэ модулийн эзэмшдэг ул мөр: `id`, `tenant_id`,
`session_id`, `user_id`, `action` (`revoked`), `actor_id`, `reason`,
`created_at`. Сесс өөрөө цөмийн `sessions`, төхөөрөмж цөмийн `devices`
хүснэгтэд байдаг — энэ модуль тэднийг **уншина** (§7-ийн хязгаар).

API `/api/v1/sso/sessions`:

| Метод | Зам | Юу хийх |
|---|---|---|
| `GET` | `/` | Идэвхтэй сессүүд (хэрэглэгч, төхөөрөмж, IP, сүүлд харагдсан) |
| `GET` | `/users/{userID}` | Нэг хүний сессүүд |
| `DELETE` | `/{id}` | Тухайн сессийг таслах |
| `DELETE` | `/users/{userID}` | Тэр хүний бүх сессийг таслах |
| `GET` | `/events` | Таслалтын түүх |

### 5.3 `accessreview` — `io.gerege.nexus.sso_access_review`

Хэн юунд хандах эрхтэйг үе үе дахин баталгаажуулах ажлын урсгал.

| | |
|---|---|
| Permission prefix | `sso_access_review` |
| Цэс | `/module/sso-access-review`, icon `clipboard-check`, order 30 |
| Хүснэгт | `sso_review_campaigns`, `sso_review_items`, `sso_review_decisions` |

`sso_review_campaigns` — нэг удаагийн хяналтын кампанит ажил: `id`,
`tenant_id`, `name`, `scope` (`all` | `app` | `role`), `scope_ref`,
`due_date`, `status` (`draft`/`open`/`closed`), `created_at/by`, `closed_at`.

`sso_review_items` — кампанит ажлын мөр бүр = нэг хүний нэг эрх: `id`,
`campaign_id`, `user_id`, `permission_code`, `role_id`, `source` (RBAC-аас
хэзээ уншсан), `status` (`pending`/`kept`/`revoked`), `reviewer_id`,
`decided_at`.

`sso_review_decisions` — шийдвэр бүрийн өөрчлөгдөшгүй бичлэг: `id`,
`item_id`, `decision`, `reviewer_id`, `note`, `created_at`. Мөр засагдсан ч
энд бичигдсэн нь үлдэнэ.

Кампанит ажил нээгдэхэд цөмийн RBAC-аас тухайн үеийн эрхийн зураглалыг
**хуулж авч** `sso_review_items` үүсгэнэ — хожим эрх өөрчлөгдвөл хянаж байсан
зүйл өөрчлөгдөхгүй.

API `/api/v1/sso/reviews`:

| Метод | Зам | Юу хийх |
|---|---|---|
| `GET` | `/campaigns` | Жагсаалт |
| `POST` | `/campaigns` | Үүсгэх (draft) |
| `POST` | `/campaigns/{id}/open` | Нээх — мөрүүдийг RBAC-аас үүсгэнэ |
| `POST` | `/campaigns/{id}/close` | Хаах |
| `GET` | `/campaigns/{id}/items` | Мөрүүд, шүүлттэй |
| `PUT` | `/items/{id}` | `kept` эсвэл `revoked` гэж шийдэх |

`revoked` нь эрхийг **автоматаар хасахгүй** — шийдвэрийг тэмдэглэнэ. Хасалт нь
цөмийн RBAC-ийн ажил бөгөөд §7-ийн хоёр дахь PR-аар л зөв холбогдоно.

### 5.4 `provisioning` — `io.gerege.nexus.sso_provisioning`

Хэрэглэгчийн бүртгэлийг холбогдсон системүүд рүү SCIM 2.0-оор түлхэх.

| | |
|---|---|
| Permission prefix | `sso_provisioning` |
| Цэс | `/module/sso-provisioning`, icon `refresh-cw`, order 40 |
| Хүснэгт | `sso_scim_targets`, `sso_scim_queue`, `sso_scim_log` |

`sso_scim_targets` — түлхэх газар: `id`, `tenant_id`, `name`, `base_url`,
`token_encrypted`, `enabled`, `sync_mode` (`push`), `created_at/by`.

`sso_scim_queue` — хүлээгдэж буй ажил: `id`, `target_id`, `op`
(`create`/`update`/`deactivate`), `user_id`, `payload` (JSONB), `attempts`,
`next_attempt_at`, `last_error`, `created_at`. Дараалал нь `FOR UPDATE SKIP
LOCKED`-оор уншигдана.

`sso_scim_log` — юу илгээгдсэн, ямар хариу ирсэн: `id`, `target_id`, `op`,
`user_id`, `status_code`, `response_excerpt`, `created_at`.

Ажиллуулагч нь модулийн доторх goroutine — `New()` дотор асаж, 30 секунд тутам
дарааллыг шалгана. Дахин оролдлого нь exponential backoff, 6 удаа.

API `/api/v1/sso/scim`:

| Метод | Зам | Юу хийх |
|---|---|---|
| `GET` | `/targets` | Жагсаалт (токен буцаахгүй) |
| `POST` | `/targets` | Нэмэх |
| `PUT` | `/targets/{id}` | Засах |
| `DELETE` | `/targets/{id}` | Устгах |
| `POST` | `/targets/{id}/test` | Холболт шалгах |
| `POST` | `/targets/{id}/resync` | Бүх хэрэглэгчийг дараалалд оруулах |
| `GET` | `/runs` | Сүүлийн ажиллагааны бүртгэл |

## 6. Схем

Бүх хүснэгт **нэг миграцад**: `db/migrations/00001_sso_schema.sql`.

Дүрмүүд:

1. Бүх нэр `sso_` угтвартай — цөмийн нэрстэй хэзээ ч мөргөлдөхгүй.
2. Цөмийн эзэмшдэг хүснэгтийг **өөрчлөхгүй**. `ALTER TABLE sessions` гэх мэт
   мөр энд байж болохгүй: цөмийн түүх энэ файлын тухай юу ч мэдэхгүй.
3. `gerege_nexus_app` рольд `GRANT SELECT, INSERT, UPDATE, DELETE` — цөмийн
   `dbguard` тенантын ажлыг тэр рольд шилжүүлдэг.
4. `tenant_id` баганатай хүснэгт бүрт индекс, RLS бодлогыг цөмийн загвараар.

Нэг өгөгдлийн санд хоёр түүх байх тул `MIGRATIONS_TABLE` заавал:
`goose_db_version_sso`. Үгүй бол цөмийн `00001` ба энэ `00001` нэг мөр болно.

## 7. Хязгаарлалт ба цөм рүү шаардлагатай хоёр PR

Энэ бол хамгийн чухал хэсэг — эдгээрийг нуувал хожим гайхах болно.

### 7.1 Federation нь login-д холбогдохгүй

Цөмийн нэвтрэлтийн урсгал (`platform/ssoclient`) итгэсэн issuer-ээ
`SSO_CLIENT_ISSUER` env-ээс уншдаг. `federation` модуль DB-д бүртгэсэн IdP нь
жинхэнэ login дээр **ашиглагдахгүй** — модуль нь бүртгэлийн гадаргуу л болно.

**Хэрэгтэй PR:** `pkg/nexus`-д `FederationSource` интерфейс нэмж,
`platform.Options`-д дамжуулдаг болгох. Цөм эхлэхдээ env-ийн оронд (эсвэл
дээр нь) тэрхүү эх сурвалжаас уншина.

Тэр PR хүртэл модулийн README ба кодод `ponytail:` тэмдэглэл үлдэнэ.

### 7.2 Sessions ба access review нь цөмийн хүснэгтийг шууд уншина

`nexus.DB` бол дурын SQL — тиймээс `sessions`, `devices`, RBAC-ийн
хүснэгтүүдийг уншиж болно, ажиллана. Гэхдээ цөмийн схем өөрчлөгдвөл энэ хоёр
модуль **чимээгүйхэн эвдэрнэ** — компайл хийгдсэн хэвээр, буруу хариу өгнө.

**Хэрэгтэй PR:** `pkg/nexus`-д уншигч API — `SessionStore` (жагсаах, таслах)
ба `RoleStore` (тухайн үеийн эрхийн зураглал). Цөмийн схем тэдний ард нуугдана.

Тэр PR хүртэл SQL нь модуль бүрийн `store.go` дотор нэг газар төвлөрнө —
өөрчлөх шаардлагатай болбол нэг файл.

### 7.3 UI

Дөрвөн модулийн цэсний бичлэг цөмийн shell дээр гарна, харин хуудас нь
`[app]/[feature]`-ийн "coming soon". Жинхэнэ хуудсууд цөмийн
`frontend/app/module/` дотор амьдардаг (appstore-ийн `registry`, `publisher`,
`store-review` яг тэнд байгаа шиг) бөгөөд энэ ажлын хүрээнд **бичигдэхгүй**.

## 8. Каталог

`catalog/apps.json` нь энэ бүтээгдэхүүний дөрвөн апп ба тэдний хажууд суух
цөмийн аппуудын **хуулбар** бичлэгүүдийг агуулна. Цөмийн аппуудын код цөмийнх;
энд зөвхөн каталогийн мөр байна.

Хуулбарлах цөмийн аппууд: `organisation`, `sso_clients`, `egov`, `documents`,
`reports`. (`sso_clients` заавал — OAuth2 клиент удирдах гадаргуу нь энэ
бүтээгдэхүүний гол хэсэг.)

Апп бүрт `catalog/manifests/<slug>.json` ба `catalog/chronicle/<slug>.json`.
Манифест нь v2.1 талбаруудтай: `publisher`, `authors`, `maintainers`,
`repository`, `homepage`, `license`, `visibility`.

**Каталог зөрвөл платформ асахаа болино** — анхааруулга биш, асалтын алдаа.
`catalog_test.go` нь энэ repo-гийн барьж чадах талыг барина: дөрвөн модулийн
компилдсэн `ID()`/`Name()`/`Version()` каталогийнхтойгоо тэнцүү эсэх.
Хуулбарласан цөмийн аппуудыг эндээс шалгах боломжгүй (код нь `internal/`
дотор) — тэднийг цөм өөрөө асахдаа шалгана. **Цөмийг bump хийх бүрд
хуулбаруудыг гараар тааруулах нь bump-ийн нэг хэсэг.**

## 9. Тест, CI, deploy

### Тест

| Файл | Юу барих |
|---|---|
| `catalog_test.go` | Каталог ↔ компилдсэн модулиуд |
| `route_policy_test.go` | Нээлттэй (session-гүй) зам байхгүй |
| `modules/*/..._test.go` | Модуль бүрийн логик; DB-тэй нь `SSO_TEST_DATABASE_URL` |

DB-тэй тест `-count=1`-ээр ажиллана.

### CI

`.github/workflows/ci.yml` нь цөмийн хуваалцсан workflow-г дууддаг:

```yaml
jobs:
  ci:
    uses: gerege-systems/open-gerege-nexus/.github/workflows/distribution-ci.yml@main
```

Дээр нь DB-тэй job: цөмийн миграцыг module cache-ээс (`go list -m -f '{{.Dir}}'`),
дараа нь энэ repo-гийнхийг `MIGRATIONS_DIR=db/migrations
MIGRATIONS_TABLE=goose_db_version_sso`-оор.

`.github/workflows/security.yml` нь `distribution-security.yml`-ийг дуудна.

`.github/dependabot.yml` — цөмийг өөрийн бүлэг болгож, шинэ tag гармагц PR
нээнэ.

### Deploy

`deploy/Dockerfile` нь appstore-ийнхтэй ижил хоёр шаттай build:

- Энэ бүтээгдэхүүний бинари, `PlatformVersion` нь `go.mod`-ын tag-аас гаргаж
  авсан (build arg биш — мартагдвал 0.0.0 болж, каталогоо уншиж чадахгүй
  асахаа болино).
- Цөмийн `cmd/migrate` ба цөмийн миграцын файлууд module cache-ээс.
- Энэ repo-гийн `db/migrations` → `/app/db/sso`.
- `catalog/` → `/app/catalog`.

`deploy/docker-compose.yml` — өөрийн `name: sso-nexus`, өөрийн postgres,
`migrate` (цөмийн) → `migrate-sso` (энэ repo-гийнх) → `backend` дараалал,
`web` нь цөмийн shell-ийг энэ origin-д зориулж барьсан образ.

`deploy/nginx/sso.gerege.mn.conf` — хуучин repo-гоос хуулж авна, домэйн хэвээр.

## 10. Хийх дараалал

1. **Аюулгүй байдлын алхам:** одоогийн `main`-ийг `archive/fork-2026-08` нэрээр
   GitHub руу түлхэх. Orphan force push хийсний дараа хуучин кодыг сэргээх
   цорын ганц зам энэ.
2. Orphan branch үүсгэж, бүх файлыг устгах.
3. Суурь: `go.mod`, `main.go` (модульгүй), `.gitignore`, `.dockerignore`,
   `LICENSE`, энэ spec файл. `go build ./...` ажиллана.
4. `db/migrations/00001_sso_schema.sql` — дөрвөн модулийн бүх хүснэгт.
5. `modules/federation` + тест.
6. `modules/sessions` + тест.
7. `modules/accessreview` + тест.
8. `modules/provisioning` + тест.
9. `main.go`-д дөрвүүлэнг бүртгэх; `catalog/` бүрэн; `catalog_test.go`,
   `route_policy_test.go`.
10. `.github/workflows/`, `.github/dependabot.yml`.
11. `deploy/`.
12. `README.md` — юу энд байгаа, юу байхгүй, §7-ийн хоёр PR-ыг нэрлэсэн.

## 11. Юуг амжилт гэж үзэх

- `go build ./...` ба `go test ./...` ногоон (DB-тэй тестүүд DB-тэй ногоон).
- `go.mod` дотор цөмийн нэг мөр; `modules/`-аас гадна Go код байхгүй.
- Цөмийн кодын нэг ч мөр хуулагдаагүй.
- `docker compose -f deploy/docker-compose.yml up` дээр инстанс асаж,
  дөрвөн апп цэсэнд гарна.
- Цөмийн шинэ tag гарахад Dependabot PR нээж, CI нь хариулна.
