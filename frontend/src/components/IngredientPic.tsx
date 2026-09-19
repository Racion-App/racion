import { useRef, useState, type MouseEvent } from "react";
import { createPortal } from "react-dom";
import { Image as ImageIcon } from "lucide-react";
import { useT } from "../i18n";

// Иконка «фото продукта» рядом с названием: по наведению или нажатию показывает картинку 160×160.
// Картинка живёт в портале с position: fixed — справа от иконки, если не помещается — слева; по вертикали
// центрируется по иконке и прижимается к краям окна. Так она не накрывает ни свою строку, ни столбец цен.

const SIZE = 160;
const GAP = 10;

export function IngredientPic({ src }: { src?: string }) {
  const { t } = useT();
  const ref = useRef<HTMLButtonElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
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
  const hide = () => setPos(null);
  const toggle = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (pos) hide();
    else show();
  };
  return (
    <>
      <button ref={ref} type="button" className="pic__btn" onClick={toggle} onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide} aria-label={t("ing.photo")} title={t("ing.photo")} aria-expanded={!!pos}>
        <ImageIcon size={13} aria-hidden />
      </button>
      {pos && createPortal(<img className="pic__float" src={src} alt="" width={SIZE} height={SIZE} style={{ left: pos.x, top: pos.y }} />, document.body)}
    </>
  );
}
