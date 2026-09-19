import { useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown, Search } from "lucide-react";
import { useT } from "../i18n";

// Кастомный выпадающий список по разметке дизайн-системы (.apple-select): кнопка-триггер, панель в <body>
// с поиском и списком. Клавиатура: стрелки, Enter, Esc, ввод букв ищет. Поиск показывается от 8 пунктов.

export type SelectOption = { value: string; label: string; sub?: string; disabled?: boolean };

type Props = {
  id?: string;
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  "aria-label"?: string;
  searchable?: boolean;
  className?: string;
};

export function Select({ id, value, options, onChange, placeholder, searchable, className, ...rest }: Props) {
  const { t } = useT();
  const uid = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [visible, setVisible] = useState(false);
  const [q, setQ] = useState("");
  const [active, setActive] = useState(0);
  const [pos, setPos] = useState<{ top: number; left: number; width: number; up: boolean }>({ top: 0, left: 0, width: 0, up: false });
  const closing = useRef<number | null>(null); // таймер закрытия: анимация 140 мс, потом размонтирование
  const withSearch = searchable ?? options.length >= 8;
  const selected = options.find((o) => o.value === value);

  const shown = useMemo(() => {
    const s = q.trim().toLowerCase();
    if (!s) return options;
    return options.filter((o) => (o.label + " " + (o.sub ?? "")).toLowerCase().includes(s));
  }, [q, options]);

  const place = () => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const below = window.innerHeight - r.bottom;
    const up = below < 280 && r.top > below;
    setPos({ top: up ? r.top - 6 : r.bottom + 6, left: r.left, width: r.width, up });
  };

  const show = () => {
    if (closing.current) {
      window.clearTimeout(closing.current);
      closing.current = null;
    }
    place();
    setQ("");
    setActive(Math.max(0, options.findIndex((o) => o.value === value)));
    setOpen(true);
    setVisible(false);
  };
  const hide = () => {
    if (closing.current) return;
    setVisible(false);
    closing.current = window.setTimeout(() => {
      closing.current = null;
      setOpen(false);
    }, 140);
    triggerRef.current?.focus();
  };

  useLayoutEffect(() => {
    if (!open) return;
    const raf = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(raf);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    if (withSearch) searchRef.current?.focus();
    const onDoc = (e: MouseEvent) => {
      if (panelRef.current?.contains(e.target as Node) || triggerRef.current?.contains(e.target as Node)) return;
      hide();
    };
    const onMove = () => place();
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("resize", onMove);
    window.addEventListener("scroll", onMove, true);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("resize", onMove);
      window.removeEventListener("scroll", onMove, true);
    };
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!open) return;
    const el = panelRef.current?.querySelector<HTMLElement>(`[data-i="${active}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [active, open]);

  const pick = (o: SelectOption) => {
    if (o.disabled) return;
    onChange(o.value);
    hide();
  };

  const onKey = (e: KeyboardEvent) => {
    if (!open) {
      if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        show();
      }
      return;
    }
    switch (e.key) {
      case "Escape":
        e.preventDefault();
        hide();
        break;
      case "ArrowDown":
        e.preventDefault();
        setActive((i) => Math.min(shown.length - 1, i + 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setActive((i) => Math.max(0, i - 1));
        break;
      case "Enter":
        e.preventDefault();
        if (shown[active]) pick(shown[active]);
        break;
      case "Home":
        setActive(0);
        break;
      case "End":
        setActive(shown.length - 1);
        break;
    }
  };

  const listId = `${uid}-list`;
  return (
    <div className={"apple-select" + (className ? " " + className : "")} onKeyDown={onKey}>
      <button
        ref={triggerRef}
        id={id}
        type="button"
        className="apple-select__trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-label={rest["aria-label"]}
        onClick={() => (open && !closing.current ? hide() : show())}
      >
        <span className={"apple-select__display" + (selected ? "" : " is-placeholder")}>{selected?.label ?? placeholder ?? ""}</span>
        <ChevronDown size={16} className="apple-select__chev" aria-hidden />
      </button>
      {open &&
        createPortal(
          <div
            ref={panelRef}
            className={"apple-select__panel" + (visible ? " is-visible" : "") + (pos.up ? " is-up" : "")}
            style={{ top: pos.up ? undefined : pos.top, bottom: pos.up ? window.innerHeight - pos.top : undefined, left: pos.left, width: Math.max(pos.width, 220) }}
          >
            {withSearch && (
              <div className="apple-select__search">
                <Search size={14} aria-hidden />
                <input
                  ref={searchRef}
                  className="apple-select__search-input"
                  value={q}
                  placeholder={t("select.search")}
                  onChange={(e) => {
                    setQ(e.target.value);
                    setActive(0);
                  }}
                  aria-label={t("select.search")}
                  autoComplete="off"
                />
              </div>
            )}
            <ul className="apple-select__list" role="listbox" id={listId} aria-activedescendant={shown[active] ? `${uid}-${active}` : undefined}>
              {shown.length === 0 && <li className="apple-select__empty">{t("select.empty")}</li>}
              {shown.map((o, i) => (
                <li
                  key={o.value}
                  id={`${uid}-${i}`}
                  data-i={i}
                  role="option"
                  aria-selected={o.value === value}
                  className={"apple-select__option" + (i === active ? " is-active" : "") + (o.value === value ? " is-selected" : "") + (o.disabled ? " is-disabled" : "")}
                  onMouseEnter={() => setActive(i)}
                  onClick={() => pick(o)}
                >
                  <span className="apple-select__label">
                    {o.label}
                    {o.sub && <small>{o.sub}</small>}
                  </span>
                  {o.value === value && <Check size={16} aria-hidden />}
                </li>
              ))}
            </ul>
          </div>,
          document.body,
        )}
    </div>
  );
}
