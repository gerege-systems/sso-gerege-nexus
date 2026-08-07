"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import brandLogo from "@/public/brand.webp";
import { usePathname, useRouter } from "next/navigation";
import { api, APP_MENU_CHANGED_EVENT } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";
import UserMenu from "@/components/UserMenu";
import AICopilot from "@/components/AICopilot";
import { Landmark, LayoutGrid, Settings, Users, Package, Boxes, Share2, CreditCard, FileText, Code2, Menu as MenuIcon, Palette, Building2, BrainCircuit, Search, Ellipsis, ShieldCheck, PenTool } from "lucide-react";

interface MenuItem { id:string; app_id?:string; app_name?:string; parent_id?:string; label:string; path?:string; icon:string; order:number }
interface AppNav { id:string; name:string; icon:string; path:string; menus:MenuItem[] }

const iconMap: Record<string, React.ReactNode> = {
  users:<Users className="w-5 h-5"/>, package:<Package className="w-5 h-5"/>, boxes:<Boxes className="w-5 h-5"/>,
  "credit-card":<CreditCard className="w-5 h-5"/>, "file-text":<FileText className="w-5 h-5"/>, code:<Code2 className="w-5 h-5"/>, landmark:<Landmark className="w-5 h-5"/>,
  "pen-tool":<PenTool className="w-5 h-5"/>,
};
// Routes that render without the ERP chrome. /oauth/consent is signed-in but
// belongs here too: it is an identity handoff to another product, and framing
// it in this one's navigation invites the user to wander off mid-flow.
const PUBLIC_ROUTES=["/","/login","/auth/eid/callback","/oauth/consent"];
const APP_ORDER=["io.example.contacts","io.example.products","io.example.inventory","io.example.billing","io.example.documents","io.example.esign","io.example.developer_portal","io.example.gov_services"];

export default function Layout({children}:{children:React.ReactNode}){
  const [menus,setMenus]=useState<MenuItem[]>([]),[user,setUser]=useState<any>(null),[loading,setLoading]=useState(true);
  const [mobileOpen,setMobileOpen]=useState(false),[mobileMoreOpen,setMobileMoreOpen]=useState(false),[panelOpen,setPanelOpen]=useState(true);
  const [query,setQuery]=useState("");
  const pathname=usePathname(),router=useRouter(),{t,locale}=useI18n(),theme=useTheme();
  const isPublic=PUBLIC_ROUTES.includes(pathname);

  useEffect(()=>setPanelOpen(localStorage.getItem("gerege_sidebar_open")!=="false"),[]);
  useEffect(()=>{if(isPublic){setLoading(false);return}void(async()=>{try{const [u,m]=await Promise.all([api.getMe(),api.getMenus()]);setUser(u);setMenus(m||[])}catch{router.push("/login")}finally{setLoading(false)}})()},[pathname,router,isPublic,locale]);
  useEffect(()=>{
    if(isPublic)return;
    const refreshMenus=()=>{void api.getMenus().then(m=>setMenus(m||[])).catch(()=>{})};
    window.addEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
    return()=>window.removeEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
  },[isPublic,locale]);
  useEffect(()=>{setMobileOpen(false);setMobileMoreOpen(false)},[pathname]);

  const apps=useMemo<AppNav[]>(()=>{
    const groups=new Map<string,MenuItem[]>();
    menus.filter(m=>m.app_id).forEach(m=>groups.set(m.app_id!,[...(groups.get(m.app_id!)||[]),m]));
    return [...groups.entries()].map(([id,items])=>{const sorted=items.sort((a,b)=>a.order-b.order),first=sorted.find(item=>item.path)!;return{id,name:first.label||first.app_name||id,icon:first.icon,path:first.path!,menus:sorted}}).sort((a,b)=>{const ai=APP_ORDER.indexOf(a.id),bi=APP_ORDER.indexOf(b.id);return (ai<0?999:ai)-(bi<0?999:bi)||a.id.localeCompare(b.id)});
  },[menus]);
  const selected=apps.find(app=>app.menus.some(m=>m.path&&pathname.startsWith(m.path)))||null;
  const platformActive=!selected;
  const searchIndex=useMemo(()=>[
    {label:t("web.menu.app_store"),app:t("web.label.platform"),path:"/apps",icon:"grid"},
    {label:t("web.menu.appearance"),app:t("web.label.platform"),path:"/settings/appearance",icon:"palette"},
    {label:t("web.menu.installed_apps"),app:t("web.label.platform"),path:"/settings/apps",icon:"settings"},
    ...apps.flatMap(app=>app.menus.filter(m=>m.path).map(m=>({label:m.label,app:app.name,path:m.path!,icon:m.icon})))
  ],[apps,locale,t]);
  const results=query.trim()?searchIndex.filter(x=>(x.label+" "+x.app).toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())).slice(0,8):[];

  function togglePanel(){if(window.matchMedia("(min-width:901px)").matches){setPanelOpen(v=>{localStorage.setItem("gerege_sidebar_open",String(!v));return !v})}else setMobileOpen(v=>!v)}
  async function logout(){try{await api.logout()}catch{}localStorage.removeItem("session_token");router.push("/login")}
  const brandTitle=selected?.name||(t("web.label.platform"));
  const mobileAppTabs=[
    {id:"platform",href:"/apps",active:platformActive,label:t("web.label.platform"),icon:<LayoutGrid className="w-5 h-5"/>},
    ...apps.map(app=>({id:app.id,href:app.path,active:selected?.id===app.id,label:app.name,icon:iconMap[app.icon]||<Package className="w-5 h-5"/>})),
  ];
  const hasMobileMore=mobileAppTabs.length>5;
  const primaryMobileTabs=hasMobileMore?mobileAppTabs.slice(0,4):mobileAppTabs;
  const remainingMobileTabs=hasMobileMore?mobileAppTabs.slice(4):[];

  if(isPublic)return <>{children}</>;
  if(loading)return <div className="min-h-screen flex items-center justify-center bg-slate-50 text-slate-500 font-medium">{t("web.message.loading_platform")}</div>;

  const platformMenus=<><MenuGroup title={t("web.group.modules")}>
    <NavLink href="/apps" active={pathname==="/apps"} icon={<LayoutGrid className="w-5 h-5"/>} label={t("web.menu.app_store")}/><NavLink href="/settings/apps" active={pathname==="/settings/apps"} icon={<Settings className="w-5 h-5"/>} label={t("web.menu.installed_apps")}/><NavLink href="/settings/ai" active={pathname==="/settings/ai"} icon={<BrainCircuit className="w-5 h-5"/>} label={t("web.menu.ai_settings")}/>
  </MenuGroup><MenuGroup title={t("web.group.settings")}>
    <NavLink href="/settings/appearance" active={pathname==="/settings/appearance"} icon={<Palette className="w-5 h-5"/>} label={t("web.menu.appearance")}/>
    <NavLink href="/settings/integrations" active={pathname==="/settings/integrations"} icon={<Share2 className="w-5 h-5"/>} label={t("web.menu.integrations")}/>
    {user?.is_admin&&<NavLink href="/settings/access" active={pathname==="/settings/access"} icon={<ShieldCheck className="w-5 h-5"/>} label={t("access.view.title")}/>}
  </MenuGroup></>;

  return <div className="gerege-shell min-h-screen flex flex-col">
    <header className="gerege-topbar h-16 flex items-center border-b sticky top-0 z-50">
      <Link href="/apps" className="gerege-header-brand w-16 h-full shrink-0 grid place-items-center border-r border-[var(--gerege-border)]">
        {theme.design==="gerege"?<img src={brandLogo.src} width={36} height={36} alt="Gerege" className="w-9 h-9 rounded-lg shadow-sm"/>:<span className="original-brand-mark w-9 h-9 rounded-lg grid place-items-center"><Building2 className="w-6 h-6"/></span>}
      </Link>
      <div className={`gerege-header-context h-full flex items-center gap-3 overflow-hidden transition-all duration-200 ${panelOpen?"is-open":""}`}>
        <span className="shrink-0 text-[var(--gerege-blue)]">{selected?(iconMap[selected.icon]||<Package className="w-5 h-5"/>):<LayoutGrid className="w-5 h-5"/>}</span>
        <span className="min-w-0"><small className="block text-[11px] leading-4 text-slate-500 truncate">Gerege ERP</small><strong className="block text-[15px] leading-5 text-slate-900 truncate">{brandTitle}</strong></span>
      </div>
      <div className="gerege-menu-toggle w-16 h-full shrink-0 grid place-items-center"><button onClick={togglePanel} className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50" aria-label={t("web.action.toggle_menu")} aria-expanded={mobileOpen}><MenuIcon className="w-5 h-5"/></button></div>
      <div className="hidden lg:flex items-center gap-2 px-4 min-w-0"><span className="gerege-session-dot w-2 h-2 rounded-full shrink-0"/><strong className="text-base text-slate-800 font-semibold truncate max-w-56">{user?.tenant_name||"Demo Tenant"}</strong></div>
      <div className="gerege-header-search hidden md:flex flex-1 items-center justify-center min-w-0 px-5 relative">
        <div className="relative w-full max-w-md"><Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400"/><input value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{if(e.key==="Enter"&&results[0]){router.push(results[0].path);setQuery("")}}} placeholder={t("web.view.search_placeholder")} className="w-full h-10 rounded-full border border-slate-200 bg-slate-100/80 pl-10 pr-4 text-sm outline-none focus:border-[var(--gerege-blue)] focus:ring-2 focus:ring-[color-mix(in_srgb,var(--gerege-blue)_15%,transparent)]"/>
          {results.length>0&&<div className="absolute top-12 inset-x-0 bg-white border border-slate-200 rounded-xl shadow-xl p-1.5 z-[70]">{results.map(item=><button key={item.path} onClick={()=>{router.push(item.path);setQuery("")}} className="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-[var(--gerege-surface-2)]"><span className="text-[var(--gerege-blue)]">{iconMap[item.icon]||<Search className="w-4 h-4"/>}</span><span className="min-w-0"><strong className="block text-sm truncate">{item.label}</strong><small className="text-slate-500 truncate">{item.app}</small></span></button>)}</div>}
        </div>
      </div>
      <div className="gerege-header-user flex pr-2 sm:pr-4 lg:pr-6"><UserMenu user={user} onLogout={logout}/></div>
    </header>

    <div className="flex flex-1 min-h-0">
      {mobileOpen&&<button className="gerege-mobile-backdrop fixed inset-0 top-16 bg-slate-950/40 z-30" aria-label={t("web.action.close_menu")} onClick={()=>setMobileOpen(false)}/>}
      <div className={`gerege-sidebar top-16 bottom-0 left-0 z-40 flex overflow-hidden ${mobileOpen?"is-mobile-open":""} ${panelOpen?"is-desktop-open":""}`}>
        <nav className="w-16 min-w-16 shrink-0 py-3 flex flex-col items-center gap-2 border-r border-[var(--gerege-border)]">
          <AppRailLink href="/apps" active={platformActive} title={t("web.label.platform")} icon={<LayoutGrid className="w-5 h-5"/>}/>
          {apps.map(app=><AppRailLink key={app.id} href={app.path} active={selected?.id===app.id} title={app.name} icon={iconMap[app.icon]||<Package className="w-5 h-5"/>}/>) }
        </nav>
        <aside className="gerege-menu-panel overflow-hidden">
          <div className="w-56 py-4"><nav className="space-y-1 px-2">
            {selected?<AppMenuGroups menus={selected.menus} pathname={pathname}/>:platformMenus}
          </nav></div>
        </aside>
      </div>
      <main className="gerege-main flex-1 p-4 sm:p-6 lg:p-8 overflow-y-auto min-w-0">{children}</main>
    </div>
    {mobileMoreOpen&&<><button className="gerege-mobile-more-backdrop" aria-label={t("web.action.close_more")} onClick={()=>setMobileMoreOpen(false)}/><section className="gerege-mobile-more-sheet" role="dialog" aria-modal="true" aria-label={t("web.view.more_apps")}><div className="gerege-mobile-more-handle"/><h2>{t("web.view.more_apps")}</h2><div className="gerege-mobile-more-grid">{remainingMobileTabs.map(tab=><MobileMoreApp key={tab.id} {...tab}/>)}</div></section></>}
    <nav className="gerege-mobile-tabs" aria-label={t("web.label.apps")}>
      {primaryMobileTabs.map(tab=><MobileAppTab key={tab.id} {...tab}/>)}
      {hasMobileMore&&<button type="button" onClick={()=>setMobileMoreOpen(v=>!v)} aria-expanded={mobileMoreOpen} className={`gerege-mobile-tab ${remainingMobileTabs.some(tab=>tab.active)||mobileMoreOpen?"is-active":""}`}><span><Ellipsis className="w-5 h-5"/></span><small>{t("web.action.more")}</small></button>}
    </nav>
    <AICopilot/>
  </div>;
}

function AppRailLink({href,active,title,icon}:{href:string;active:boolean;title:string;icon:React.ReactNode}){return <Link href={href} title={title} aria-label={title} className={`w-11 h-11 rounded-xl grid place-items-center transition ${active?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] shadow-sm":"text-slate-500 hover:bg-[var(--gerege-surface-2)] hover:text-slate-800"}`}>{icon}</Link>}
function MobileAppTab({href,active,label,icon}:{href:string;active:boolean;label:string;icon:React.ReactNode}){return <Link href={href} aria-label={label} aria-current={active?"page":undefined} className={`gerege-mobile-tab ${active?"is-active":""}`}><span>{icon}</span><small>{label}</small></Link>}
function MobileMoreApp({href,active,label,icon}:{href:string;active:boolean;label:string;icon:React.ReactNode}){return <Link href={href} aria-current={active?"page":undefined} className={`gerege-mobile-more-app ${active?"is-active":""}`}><span>{icon}</span><strong>{label}</strong></Link>}
function NavLink({href,active,icon,label}:{href:string;active:boolean;icon:React.ReactNode;label:string}){return <Link href={href} className={`gerege-nav-link flex items-center gap-3 px-3 py-2.5 text-sm font-medium transition ${active?"gerege-nav-link-active font-semibold":""}`}><span className="gerege-nav-icon">{icon}</span><span>{label}</span></Link>}
function MenuGroup({title,children}:{title:string;children:React.ReactNode}){return <section className="mb-6"><h3 className="px-3 mb-2 text-[11px] font-bold uppercase tracking-wider text-slate-400">{title}</h3><div className="space-y-1">{children}</div></section>}
function AppMenuGroups({menus,pathname}:{menus:MenuItem[];pathname:string}){const roots=menus.filter(item=>!item.parent_id).sort((a,b)=>a.order-b.order);return <>{roots.map(root=><MenuGroup key={root.id} title={root.label}>{menus.filter(item=>item.parent_id===root.id&&item.path).sort((a,b)=>a.order-b.order).map(item=><NavLink key={item.id} href={item.path!} active={pathname.startsWith(item.path!)} icon={iconMap[item.icon]||<Package className="w-5 h-5"/>} label={item.label}/>)}</MenuGroup>)}</>}
