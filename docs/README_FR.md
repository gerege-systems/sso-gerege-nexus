# Gerege SSO

**Plateforme d'authentification unique et de gestion des identités et des accès**

**Gerege SSO** est une plateforme open source, bâtie sur
[Gerege Nexus](https://github.com/gerege-systems/open-gerege-nexus), qui unifie
les utilisateurs, l'authentification, les droits et l'accès aux systèmes des
organisations publiques et privées. Elle place le **mongol au premier plan** et
s'intègre directement à l'infrastructure numérique nationale de la Mongolie
(DAN, E-ID, XYP / ХУР).

Un citoyen ou un agent se vérifie **une seule fois** avec son identité numérique
nationale, puis accède à toutes les applications auxquelles il a droit sans se
reconnecter. Les systèmes tiers se raccordent à cette session via OAuth2 / OIDC
plutôt que de tenir leurs propres bases de mots de passe.

*Nexus* désigne le point de connexion : là où se rejoignent organisations,
services, processus, systèmes, utilisateurs et données. Gerege SSO en est la
**couche d'identité et d'accès** : elle établit qui est la personne et décide de
ce à quoi elle peut accéder. Ce sont les modules qui y tournent qui donnent son
caractère à un déploiement.

Les modules sont compilés dans un seul binaire Go, tandis qu'un magasin
d'applications adossé à PostgreSQL décide des applications actives pour chaque
locataire — une séparation modulaire sans les appels réseau ni le coût
d'exploitation des microservices.

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <a href="README_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> <b>Français</b>
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

## Sommaire

- [Auteurs](#auteurs)
- [Capacités principales](#capacités-principales)
- [Applications métier](#applications-métier)
- [Structure du dépôt](#structure-du-dépôt)
- [Démarrage](#démarrage)
- [Configuration](#configuration)
- [Aperçu de l'API](#aperçu-de-lapi)
- [Tests et contrôles qualité](#tests-et-contrôles-qualité)
- [Sécurité](#sécurité)
- [Index de la documentation](#index-de-la-documentation)

---

## Auteurs

| Contributeur | Rôle |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | Architecture, cœur de la plateforme |
| **Gemini AI** | Génération de code, documentation |
| **Claude AI** | Analyse de code, audit de sécurité |

---

## Capacités principales

### 1. Monolithe modulaire haute performance

- **Modules Go compilés** — `contacts`, `products`, `inventory`, `billing`,
  `documents` et `developer_portal` sont compilés dans un binaire unique et
  appelés en processus.
- **Magasin d'applications par locataire** — droits applicatifs, menus et RBAC
  sont pilotés depuis PostgreSQL (`app_installations`).
- **Résolveur de dépendances** — résolution récursive sur un graphe orienté
  acyclique, avec détection de cycles et vérification des contraintes semver.
- **Synchronisation du catalogue** — `catalog/apps.json` fait autorité ; la
  table `apps` est réconciliée à chaque démarrage.

### 2. Moteur de résilience cloud-native

| Module | Rôle |
| --- | --- |
| `resilience/breaker.go` | Disjoncteur adaptatif inspiré du SRE Google |
| `resilience/loadshedder.go` | Délestage avec `503` + `Retry-After` sous charge |
| `resilience/singleflight.go` | Fusionne les traitements identiques en vol |
| `resilience/retry.go` | Réessai avec backoff exponentiel |

### 3. Infrastructure numérique nationale

- **XYP — échange d'informations de l'État** (`platform/gerege/xyp.go`) :
  registre civil des citoyens (`WS100101`) et vérification des personnes
  morales (`WS100201`).
- **E-ID national et DAN** ([`developer.gerege.mn`](https://developer.gerege.mn),
  [`eidmongolia.mn`](https://eidmongolia.mn)) — signature numérique PKI, OTP
  mobile, SSO bancaire et vérification faciale biométrique.
- **Fournisseur OAuth2 / OIDC intégré**
  (`/.well-known/openid-configuration`) délivrant des jetons
  client-credentials à des systèmes tiers.

> **Remarque.** Le mode simulé (mock) pour E-ID, DAN et XYP est une commodité de
> développement uniquement. Avec `ENVIRONMENT=production` il est désactivé
> automatiquement : un numéro d'enregistrement fabriqué ne peut jamais
> authentifier.

### 4. Copilote IA et analytique

- **Assistant IA** (`platform/ai/copilot.go`) — conversation classée par
  intention, branchée sur les données réelles du locataire.
- **Prévision de la demande** (`platform/ai/inventory_forecaster.go`) —
  recommandations de stock de sécurité et de point de commande à partir de
  l'historique des mouvements.

---

## Applications métier

| # | Application | ID | Route | Description |
| --- | --- | --- | --- | --- |
| 1 | Contacts | `io.example.contacts` | `/contacts` | Répertoire clients et fournisseurs avec préremplissage XYP |
| 2 | Produits | `io.example.products` | `/products` | Catalogue, tarifs et SKU par locataire |
| 3 | Stocks | `io.example.inventory` | `/inventory` | Entrepôts, niveaux de stock, journal des mouvements |
| 4 | Facturation & e-Barimt | `io.example.billing` | `/billing` | Facturation, TVA 10 %, reçus e-Barimt |
| 5 | Documents & signature électronique | `io.example.documents` | `/documents` | Circulation des documents, signatures, approbations |
| 6 | Portail développeur & SSO OAuth2 | `io.example.developer_portal` | `/developer/apps` | Enregistrement des clients OAuth2 |

Les routes ne s'ouvrent qu'une fois l'application installée et activée pour le
locataire ; sinon le contrôle renvoie `403 Forbidden`.

---

## Structure du dépôt

```
backend/
  cmd/api/            Serveur d'API HTTP (+ jeu de données de démonstration)
  cmd/migrate/        Exécuteur de migrations Goose
  db/migrations/      Migrations SQL
  internal/
    module.go         Le contrat de module Go
    apps/             Modules métier
    platform/         Services du cœur de plateforme
frontend/             Client web Next.js 15 (App Router)
catalog/              Catalogue et manifestes du magasin d'applications
deploy/               Dockerfile de production, configuration Nginx
docs/                 Documentation et traductions
```

---

## Démarrage

### Prérequis

- Go 1.25+
- Node.js 20+
- PostgreSQL 16+ (ou Docker Compose)

### 1. Docker Compose

```bash
docker compose up -d
```

Les migrations s'exécutent dans un service `migrate` dédié, à usage unique,
avant le démarrage de l'API.

### 2. Manuellement

**Backend :**

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

**Frontend :**

```bash
cd frontend
npm ci
npm run dev
```

Ouvrez [http://localhost:3000](http://localhost:3000).

### Identifiants de démonstration

| Champ | Valeur |
| --- | --- |
| E-mail | `admin@example.com` |
| Mot de passe | `Password123!` |
| Locataire | `Demo Corporation` (`slug: demo`) |

Le compte de démonstration n'est créé qu'en dehors de la production. En
production il n'est créé que si `SEED_DEMO_DATA=true` est défini explicitement.

---

## Déploiement automatisé

Une exécution réussie de [`ci.yml`](../.github/workflows/ci.yml) sur `main`
déclenche [`deploy.yml`](../.github/workflows/deploy.yml) — et non la poussée
elle-même, de sorte qu'un commit dont les tests ont échoué n'est jamais
déployé :

1. Construire et publier les images backend et frontend sur GHCR (`:latest` et `:<sha>`).
2. Copier `docker-compose.prod.yml` sur le serveur.
3. Écrire le `.env` du serveur depuis les secrets GitHub et récupérer les images.
4. Exécuter les migrations jusqu'au bout, puis basculer l'API et le frontend.
5. Sonder `/health` et `/ready`, afficher les journaux des conteneurs et faire
   échouer l'exécution si le déploiement n'est pas sain.

Déploiement manuel : Actions → *Deploy to Production* → **Run workflow**, en
épinglant éventuellement une étiquette d'image.

Secrets requis dans le dépôt :

| Secret | Requis | Description |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Oui | Clé privée de l'utilisateur de déploiement. Sans elle, le déploiement est ignoré |
| `POSTGRES_PASSWORD` | Oui | Mot de passe de la base de données sur le serveur |
| `SSO_DEFAULT_CLIENT_SECRET` | Oui | Obligatoire pour le client OAuth2 intégré en production |
| `DEPLOY_HOST` | **Oui** | Aucune valeur par défaut |
| `PUBLIC_ORIGIN` (variable de dépôt) | **Oui** | Aucune valeur par défaut |
| `DEPLOY_USER` / `DEPLOY_PORT` | Non | Par défaut `deploy` / `22` |

> Ce dépôt est un fork de `open-gerege-nexus` et **partage un serveur** avec
> lui : celui-ci répond sur `sso.gerege.mn`, son voisin sur `nexus.gerege.mn`.
> C'est pourquoi `DEPLOY_HOST` et `PUBLIC_ORIGIN` n'ont **aucune valeur par
> défaut** : un secret oublié arrête le workflow au lieu de déployer ce dépôt
> par-dessus la production du voisin. `PUBLIC_ORIGIN` définit en un seul endroit
> le CORS, l'émetteur OIDC et le callback eID : le déplacer entraîne donc le
> DNS, le certificat TLS et tout client ayant épinglé l'émetteur.

Le serveur n'a besoin que de Docker — ni code source, ni chaîne d'outils
Go/Node. Voir [`deploy/.env.prod.example`](../deploy/.env.prod.example) pour les
valeurs.

---

## Configuration

Voir [`.env.example`](../.env.example) pour la liste complète.

| Variable | Défaut | Description |
| --- | --- | --- |
| `DATABASE_URL` | localhost | Chaîne de connexion PostgreSQL |
| `PORT` | `8080` | Port d'écoute de l'API |
| `ENVIRONMENT` | `development` | `production` active les valeurs durcies |
| `APP_CATALOG_PATH` | `catalog/apps.json` | Chemin du catalogue du magasin d'applications |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | Liste d'origines autorisées (CORS) |
| `TRUST_PROXY_HEADERS` | `false` | Faut-il faire confiance à `X-Forwarded-For` |
| `SEED_DEMO_DATA` | activé hors production | Créer le compte de démonstration |
| `SSO_DEFAULT_CLIENT_SECRET` | — | Requis en production |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | activé hors production | Simuler les intégrations nationales |

---

## Aperçu de l'API

| Méthode | Chemin | Description |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | Sondes de vivacité et de disponibilité |
| `GET` | `/metrics` | Métriques Prometheus |
| `POST` | `/api/v1/auth/login` | Connexion par e-mail et mot de passe |
| `POST` | `/api/v1/auth/eid/login` | Connexion par E-ID national |
| `POST` | `/api/v1/auth/dan/login` | Connexion via la passerelle DAN |
| `POST` | `/api/v1/auth/logout` | Révoquer la session |
| `GET` | `/api/v1/menus` | Menus des applications activées pour le locataire |
| `GET` | `/api/v1/store/apps` | Liste du magasin d'applications |
| `POST` | `/api/v1/store/apps/{slug}/install` | Installer une application (admin) |
| `POST` | `/oauth2/token` | Jeton OAuth2 client credentials |

Les jetons de session circulent soit dans le cookie HttpOnly, soit via
`Authorization: Bearer <token>`.

---

## Tests et contrôles qualité

```bash
# Tests unitaires backend avec le détecteur de courses
cd backend && go test -race ./...

# Analyse statique
cd backend && go vet ./... && golangci-lint run

# Analyse des vulnérabilités
cd backend && govulncheck ./...

# Build du frontend
cd frontend && npm run build
```

La CI exécute le lint, les tests, le build du frontend, la construction de
l'image Docker, govulncheck et gosec à chaque poussée et chaque pull request.

---

## Sécurité

- Les jetons de session sont des valeurs aléatoires de 256 bits ; seul leur
  condensé SHA-256 est stocké.
- Les mots de passe sont hachés avec bcrypt et les tentatives de connexion sont
  limitées par IP.
- Installer, activer ou désactiver des applications et enregistrer des
  intégrations exige les droits d'administrateur du locataire.
- L'authentification des clients OAuth2 utilise une comparaison à temps
  constant.

Signalez les vulnérabilités comme décrit dans [`SECURITY.md`](../SECURITY.md).

---

## Index de la documentation

| Document | Description |
| --- | --- |
| [Centre de documentation](README.md) | Index de tous les documents et traductions |
| [Spécification d'architecture](ARCHITECTURE_SPECIFICATION.md) | Couches de la plateforme et décisions de conception |
| [Guide de création de module](MODULE_AUTHORING_GUIDE.md) | Comment construire un nouveau module applicatif |
| [Contribuer](../CONTRIBUTING.md) | Processus de contribution |
| [Politique de sécurité](../SECURITY.md) | Signalement des vulnérabilités |
| [Code de conduite](../CODE_OF_CONDUCT.md) | Règles de la communauté |
| [Journal des modifications](../CHANGELOG.md) | Historique des versions |

---

## Remerciements et inspirations

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   de **[@snykk](https://github.com/snykk)** — fondations de l'API REST Go.
2. **[Odoo](https://github.com/odoo/odoo)** — magasin d'applications modulaire
   et modèle de dépendances.
3. **[go-zero](https://github.com/zeromicro/go-zero)** — moteur de résilience
   cloud-native.

---

## Licence

Copyright (c) 2026 **Gerege Systems Development Team, Gemini AI &
Claude AI**. Distribué sous licence Apache 2.0 — voir
[`LICENSE`](../LICENSE).

Icônes de drapeaux par [Flaticon](https://www.flaticon.com/)
([attribution](assets/icons/ATTRIBUTION.md)).
