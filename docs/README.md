# Баримт бичгийн төв — Documentation Hub

Энэ хавтас нь **Gerege SSO**-ын бүх баримт бичиг болон орчуулгыг
агуулна. Үндсэн хэл нь монгол; орчуулгууд нь файлын нэрийн `_AR`, `_ZH`, `_EN`,
`_FR`, `_RU`, `_ES` дагаварт хадгалагдана.

**Хэлний бодлого: монгол хэл + НҮБ-ын албан ёсны 6 хэл** (араб, хятад, англи,
франц, орос, испани) — нийт 7 хэл. Монгол хэл эх сурвалж, бусад нь орчуулга.
Шинэ хэл нэмэхийн өмнө энэ бодлогыг өөрчлөх шаардлагатай: жагсаалт нь дур
зоргоор биш, олон улсын байгууллагуудын хэрэглэдэг жишигт тулгуурласан.

This directory holds every Gerege SSO document and translation. Mongolian is
the source language. **The language policy is Mongolian plus the six official
languages of the United Nations** (Arabic, Chinese, English, French, Russian,
Spanish) — seven in total.

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
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

---

## Танилцуулга — Overview

| Хэл | Файл |
| --- | --- |
| Монгол (эх сурвалж) | [`../README.md`](../README.md) |
| العربية | [`README_AR.md`](README_AR.md) |
| 中文 | [`README_ZH.md`](README_ZH.md) |
| English | [`README_EN.md`](README_EN.md) |
| Français | [`README_FR.md`](README_FR.md) |
| Русский | [`README_RU.md`](README_RU.md) |
| Español | [`README_ES.md`](README_ES.md) |

## Техникийн баримт — Technical documentation

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`ARCHITECTURE_SPECIFICATION.md`](ARCHITECTURE_SPECIFICATION.md) | MN | Платформын давхаргууд, өгөгдлийн загвар, архитектурын шийдвэрүүд |
| [`ARCHITECTURE_SPECIFICATION_EN.md`](ARCHITECTURE_SPECIFICATION_EN.md) | EN | Architecture specification |
| [`MODULE_AUTHORING_GUIDE.md`](MODULE_AUTHORING_GUIDE.md) | EN | Шинэ апп модуль хөгжүүлэх алхам алхмаар заавар |
| [`GOV_SERVICES_WORKFLOW.md`](GOV_SERVICES_WORKFLOW.md) | EN | Тохируулж болох төрийн үйлчилгээний урсгал, шилжүүлэлт, баталгаажуулалт |

## Төслийн журам — Project governance

| Баримт | Хэл | Тайлбар |
| --- | --- | --- |
| [`../CONTRIBUTING.md`](../CONTRIBUTING.md) | MN | Хувь нэмэр оруулах журам |
| [`CONTRIBUTING_EN.md`](CONTRIBUTING_EN.md) | EN | Contribution guide |
| [`../CODE_OF_CONDUCT.md`](../CODE_OF_CONDUCT.md) | MN | Ёс зүйн дүрэм |
| [`CODE_OF_CONDUCT_EN.md`](CODE_OF_CONDUCT_EN.md) | EN | Code of conduct |
| [`../SECURITY.md`](../SECURITY.md) | MN | Аюулгүй байдлын бодлого |
| [`SECURITY_EN.md`](SECURITY_EN.md) | EN | Security policy |
| [`../CHANGELOG.md`](../CHANGELOG.md) | EN | Өөрчлөлтийн түүх |

---

## Орчуулга нэмэх — Adding a translation

1. Эх баримтыг хуулж, файлын нэрэнд ISO 639-1 хэлний код бүхий дагавар нэмнэ:
   `README_JA.md`, `CONTRIBUTING_JA.md` гэх мэт.
2. Баримтын эхэнд, оршил догол мөрийн дараа, badge-үүдийн өмнө хэлний
   сонголтын мөрийг байрлуулна. Туг бүрийн зураг
   [`assets/icons/`](assets/icons/)-д хадгалагдана.
3. Бүх хэлний хувилбар дээрх сонголтын мөрийг шинэ хэлээр нөхнө — сонголт нь
   хэлүүдийн хооронд тэгш хэмтэй байх ёстой.
4. Энэ индекс файлын хүснэгтэд шинэ мөр нэмнэ.

Хэлний сонголтын мөрийн загвар (`docs/` доторх файлд):

```html
<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
</p>
```

Идэвхтэй байгаа хэлийг холбоосгүй, `<b>` тэгээр тодруулна.

---

## Дүрсний эх сурвалж — Icon source

Тугны дүрсийг [Flaticon](https://www.flaticon.com/)-оос авч репод хадгалсан.
Дэлгэрэнгүйг [`assets/icons/ATTRIBUTION.md`](assets/icons/ATTRIBUTION.md)-ээс
үзнэ үү. Баримт бичигт emoji дүрс ашиглахгүй — бүх дүрс нь Flaticon-ы дүрсийн
сангаас авсан зураг байна.
