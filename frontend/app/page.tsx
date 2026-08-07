"use client";

import Link from "next/link";
import EIDLogin from "@/components/EIDLogin";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {TranslationKey, useI18n} from "@/lib/i18n";
import {ArrowRight,CheckCircle2,Fingerprint,KeyRound,Layers,Network,ShieldCheck,Waypoints} from "lucide-react";

const features:{icon:typeof Fingerprint;title:TranslationKey;body:TranslationKey}[]=[
  {icon:Fingerprint,title:"website.feature.instant_title",body:"website.feature.instant_body"},
  {icon:Network,title:"website.feature.sso_title",body:"website.feature.sso_body"},
  {icon:ShieldCheck,title:"website.feature.passwordless_title",body:"website.feature.passwordless_body"},
  {icon:Waypoints,title:"website.feature.channels_title",body:"website.feature.channels_body"},
];
const trustPoints:TranslationKey[]=["website.trust.cookie","website.trust.rbac","website.trust.allowlist","website.trust.audit"];

export default function LandingPage(){const {t}=useI18n();return <div className="gp-landing" id="top">
  <header className="gp-nav"><a href="#top" className="gp-brand"><img src="/brand.webp" alt=""/><span>Gerege SSO</span></a><nav><a href="#features">{t("website.menu.features")}</a><a href="#trust">{t("website.menu.trust")}</a><a href="#technology">{t("website.menu.technology")}</a></nav><div className="gp-actions"><LanguageSwitcher variant="dark"/><Link href="/login" className="gp-gold">{t("website.action.sign_in")}</Link></div></header>
  <main>
    <section className="gp-hero"><div className="gp-pattern"/><div className="gp-hero__inner"><div className="gp-copy"><span className="gp-eyebrow"><i/> SSO · eID · OIDC</span><h1>{t("website.view.hero_title_lead")} <em>{t("website.view.hero_title_highlight")}</em> {t("website.view.hero_title_tail")}</h1><p>{t("website.view.hero_lede")}</p><div className="gp-cta"><a href="#eid-login" className="gp-gold gp-gold--large">{t("website.action.eid_sign_in")} <ArrowRight/></a><a href="#features" className="gp-outline">{t("website.action.see_features")}</a></div><div className="gp-stats"><span><b>eID</b>{t("website.stat.eid")}</span><span><b>OAuth2 · OIDC</b>{t("website.stat.standards")}</span><span><b>SSO</b>{t("website.stat.sso")}</span></div></div><div id="eid-login" className="gp-login-slot"><EIDLogin compact/></div></div></section>
    <section className="gp-section" id="features"><div className="gp-heading"><span>{t("website.view.features_eyebrow")}</span><h2>{t("website.view.features_title")}</h2><p>{t("website.view.features_lede")}</p></div><div className="gp-grid">{features.map(({icon:Icon,title,body},i)=><article key={title} className={i===1?"gp-feature gp-feature--dark":"gp-feature"}><Icon/><h3>{t(title)}</h3><p>{t(body)}</p></article>)}</div></section>
    <section className="gp-trust" id="trust"><div><span className="gp-eyebrow gp-eyebrow--blue"><i/> {t("website.view.trust_eyebrow")}</span><h2>{t("website.view.trust_title")}</h2><p>{t("website.view.trust_lede")}</p></div><ul>{trustPoints.map(point=><li key={point}><CheckCircle2/>{t(point)}</li>)}</ul></section>
    <section className="gp-tech" id="technology"><div><Layers/><h3>Gerege SSO</h3><p>{t("website.tech.erp_body")}</p></div><ArrowRight/><div><Fingerprint/><h3>eID Mongolia</h3><p>{t("website.tech.eid_body")}</p></div><ArrowRight/><div><KeyRound/><h3>OIDC / SSO</h3><p>{t("website.tech.sso_body")}</p></div></section>
  </main><footer className="gp-footer"><span>© 2026 Gerege Systems · Gerege SSO</span><span>{t("website.message.footer_note")}</span></footer>
</div>}
