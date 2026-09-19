import { useRef, useState, type MouseEvent } from "react";
import { createPortal } from "react-dom";
import { useT } from "../i18n";

// Миниатюра продукта рядом с названием: по наведению или нажатию раскрывается до 160×160.
// Картинка живёт в портале с position: fixed — справа от иконки, если не помещается — слева; по вертикали
// центрируется по иконке и прижимается к краям окна. Так она не накрывает ни свою строку, ни столбец цен.
// Картинка — popover (top layer), поэтому видна и над <dialog>, а портал в body не растягивает лист по горизонтали.

const SIZE = 160;
const GAP = 10;

function openPopover(el: HTMLImageElement | null) {
  try {
    el?.showPopover();
  } catch {
    // браузер без Popover API: картинка и так position: fixed
  }
}

export function IngredientPic({ src }: { src?: string }) {
  const { t } = useT();
  const ref = useRef<HTMLButtonElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const by = useRef<"hover" | "focus" | "click" | null>(null);
  if (!src) return null;
  const show = () => {
    const r = ref.current?.getBoundingClientRect();
    if (!r) return;
    let x = r.right + GAP;
    if (x + SIZE > window.innerWidth - 8) x = r.left - GAP - SIZE;
    if (x < 8) x = 8;
    const y = Math.min(Math.max(8, r.top + r.height / 2 - SIZE / 2), window.innerHeight - SIZE - 8);
    setPos({ x, y });
  };
  const hide = () => {
    by.current = null;
    setPos(null);
  };
  // На телефоне тап даёт mouseenter и focus раньше click: картинка успевала показаться и тут же гасла.
  // Поэтому помним, чем открыли: click закрывает только то, что открыл click; уход мыши гасит только hover.
  const open = (how: "hover" | "focus" | "click") => {
    if (by.current !== "click") by.current = how;
    show();
  };
  const toggle = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (pos && by.current === "click") hide();
    else open("click");
  };
  const leave = () => {
    if (by.current === "hover") hide();
  };
  return (
    <>
      <button ref={ref} type="button" className="pic__btn" onClick={toggle} onMouseEnter={() => open("hover")} onMouseLeave={leave} onFocus={() => open("focus")} onBlur={hide} aria-label={t("ing.photo")} title={t("ing.photo")} aria-expanded={!!pos}>
        <img className="pic__thumb" src={src} alt="" width={22} height={22} loading="lazy" decoding="async" />
      </button>
      {pos && createPortal(<img ref={openPopover} className="pic__float" popover="manual" src={src} alt="" width={SIZE} height={SIZE} style={{ left: pos.x, top: pos.y }} />, document.body)}
    </>
  );
}
