import { useEffect, useRef, type ReactNode } from "react";
import { X } from "lucide-react";

// Нижний лист на нативном <dialog>: фокус-трап, Esc и клик по фону — бесплатно.
export function Sheet({ open, onClose, title, children, closeLabel }: { open: boolean; onClose: () => void; title: string; children: ReactNode; closeLabel?: string }) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  // Страница под листом не должна прокручиваться: фиксируем body на текущей позиции (работает и в iOS Safari).
  useEffect(() => {
    if (!open) return;
    const y = window.scrollY;
    const body = document.body;
    const html = document.documentElement;
    const prev = { position: body.style.position, top: body.style.top, width: body.style.width, overflow: body.style.overflow, pad: body.style.paddingRight, htmlOverflow: html.style.overflowY };
    // Ширина скроллбара: если браузер не умеет scrollbar-gutter, компенсируем отступом, чтобы страница не прыгала.
    const gutter = window.innerWidth - html.clientWidth;
    const stable = typeof CSS !== "undefined" && CSS.supports("scrollbar-gutter", "stable");
    body.style.position = "fixed";
    body.style.top = `-${y}px`;
    body.style.width = "100%";
    body.style.overflow = "hidden";
    if (!stable && gutter > 0) body.style.paddingRight = `${gutter}px`;
    return () => {
      body.style.position = prev.position;
      body.style.top = prev.top;
      body.style.width = prev.width;
      body.style.overflow = prev.overflow;
      body.style.paddingRight = prev.pad;
      html.style.overflowY = prev.htmlOverflow;
      window.scrollTo({ top: y, behavior: "instant" as ScrollBehavior });
    };
  }, [open]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const onCancel = (e: Event) => {
      e.preventDefault();
      onClose();
    };
    const onClick = (e: MouseEvent) => {
      if (e.target === el) onClose();
    };
    el.addEventListener("cancel", onCancel);
    el.addEventListener("click", onClick);
    return () => {
      el.removeEventListener("cancel", onCancel);
      el.removeEventListener("click", onClick);
    };
  }, [onClose]);

  return (
    <dialog ref={ref} className="bsheet" aria-label={title}>
      <div className="bsheet__grip" aria-hidden />
      <button type="button" className="bsheet__close" onClick={onClose} aria-label={closeLabel ?? "Закрыть"}>
        <X size={18} aria-hidden />
      </button>
      <div className="bsheet__body">{children}</div>
    </dialog>
  );
}
