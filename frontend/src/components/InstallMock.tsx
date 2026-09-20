import type { ReactNode } from "react";
import { Book, Bookmark, ChevronLeft, ChevronRight, Copy, Download, EllipsisVertical, LayoutGrid, Mic, MoreHorizontal, Plus, RotateCw, Search, Share, SquarePlus, Star, X } from "lucide-react";
import { useT } from "../i18n";

// Макеты экранов для инструкции по установке: панель Safari с меню «…», лист «Поделиться», диалог «На экран
// „Домой“», меню Chrome на Android и на ПК. Собраны из HTML и CSS, а не из чужих скриншотов, поэтому подписи
// системных кнопок переводятся вместе с интерфейсом. Синяя рамка (.mock__hl) — куда нажимать; порядок рамок
// внутри одного макета = порядок нажатий. Размер 4:5, всё в em от базового кегля, чтобы масштабироваться.

export type MockKind = "menu26" | "share18" | "addhome" | "add" | "amenu" | "ainstall" | "aconfirm" | "omnibox" | "dconfirm";

function Row({ icon, text, hl, chevron }: { icon: ReactNode; text: string; hl?: boolean; chevron?: boolean }) {
  return (
    <div className={"mock__row" + (hl ? " mock__hl" : "")}>
      <span className="mock__ico">{icon}</span>
      <span className="mock__txt">{text}</span>
      {chevron && <ChevronRight size={10} className="mock__chev" aria-hidden />}
    </div>
  );
}

function AppHead({ name, url }: { name: string; url: string }) {
  return (
    <div className="mock__app">
      <span className="mock__icon"><i /><i /><i /></span>
      <span><b>{name}</b><small>{url}</small></span>
    </div>
  );
}

export function InstallMock({ kind }: { kind: MockKind }) {
  const { t } = useT();
  const url = "racion.app";
  const name = t("brand");
  return (
    <span className={"mock mock--" + kind} aria-hidden>
      {kind === "menu26" && (
        <>
          <div className="mock__page" />
          <div className="mock__menu mock__menu--up">
            <Row icon={<Share size={10} />} text={t("mock.share")} hl />
            <Row icon={<Copy size={10} />} text={t("mock.copy")} />
            <Row icon={<Mic size={10} />} text={t("mock.voice")} />
          </div>
          <div className="mock__bar">
            <span className="mock__pill"><Search size={9} /> {url} <RotateCw size={9} /></span>
            <span className="mock__round mock__hl"><MoreHorizontal size={12} /></span>
          </div>
        </>
      )}
      {kind === "share18" && (
        <>
          <div className="mock__page" />
          <div className="mock__omni"><span className="mock__pill mock__pill--wide">{url}</span></div>
          <div className="mock__tools">
            <ChevronLeft size={13} /><ChevronRight size={13} />
            <span className="mock__hl mock__tool"><Share size={13} /></span>
            <Book size={13} /><LayoutGrid size={13} />
          </div>
        </>
      )}
      {kind === "addhome" && (
        <div className="mock__sheet">
          <AppHead name={name} url={url} />
          <div className="mock__list">
            <Row icon={<Bookmark size={10} />} text={t("mock.bookmark")} />
            <Row icon={<Star size={10} />} text={t("mock.favorite")} />
            <Row icon={<Search size={10} />} text={t("mock.find")} />
            <Row icon={<SquarePlus size={10} />} text={t("mock.addhome")} hl />
          </div>
        </div>
      )}
      {kind === "add" && (
        <div className="mock__dialog">
          <div className="mock__dhead">
            <span className="mock__round mock__round--sm"><X size={9} /></span>
            <span className="mock__dtitle">{t("mock.addhome.title")}</span>
            <span className="mock__btn mock__hl">{t("mock.add")}</span>
          </div>
          <AppHead name={name} url={"https://" + url} />
          <div className="mock__toggle"><span>{t("mock.webapp")}</span><i /></div>
        </div>
      )}
      {kind === "amenu" && (
        <>
          <div className="mock__abar"><span className="mock__pill mock__pill--wide">{url}</span><span className="mock__hl mock__tool"><EllipsisVertical size={12} /></span></div>
          <div className="mock__page mock__page--light" />
        </>
      )}
      {kind === "ainstall" && (
        <>
          <div className="mock__abar"><span className="mock__pill mock__pill--wide">{url}</span><EllipsisVertical size={12} /></div>
          <div className="mock__page mock__page--light" />
          <div className="mock__menu mock__menu--right">
            <Row icon={<Plus size={10} />} text={t("mock.newtab")} />
            <Row icon={<Star size={10} />} text={t("mock.bookmarks")} />
            <Row icon={<Download size={10} />} text={t("mock.install")} hl />
          </div>
        </>
      )}
      {(kind === "aconfirm" || kind === "dconfirm") && (
        <>
          <div className="mock__page mock__page--light" />
          <div className="mock__confirm">
            <b>{t("mock.install.q")}</b>
            <AppHead name={name} url={url} />
            <div className="mock__actions"><span className="mock__btn mock__hl">{t("mock.install.ok")}</span></div>
          </div>
        </>
      )}
      {kind === "omnibox" && (
        <>
          <div className="mock__abar"><span className="mock__pill mock__pill--wide">{url}</span><span className="mock__hl mock__tool"><Download size={11} /></span></div>
          <div className="mock__page mock__page--light" />
        </>
      )}
    </span>
  );
}
