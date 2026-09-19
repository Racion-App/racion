import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { useT } from "../i18n";

// Свой confirm вместо системного: центрированный <dialog> в духе алерта iOS — заголовок, пояснение,
// две кнопки (опасное действие красным). Вызов из любого места: const confirm = useConfirm();
// if (!(await confirm({ title, text, ok, danger: true }))) return;

export type ConfirmOptions = { title: string; text?: string; ok?: string; cancel?: string; danger?: boolean };
type Ask = (o: ConfirmOptions) => Promise<boolean>;

const Ctx = createContext<Ask>(() => Promise.resolve(false));

export function useConfirm(): Ask {
  return useContext(Ctx);
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const { t } = useT();
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolver = useRef<((v: boolean) => void) | null>(null);
  const ref = useRef<HTMLDialogElement>(null);

  const ask = useCallback<Ask>((o) => {
    resolver.current?.(false); // предыдущий вопрос, если был, считаем отменённым
    setOpts(o);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);
  const answer = useCallback((v: boolean) => {
    resolver.current?.(v);
    resolver.current = null;
    setOpts(null);
  }, []);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (opts && !el.open) el.showModal();
    if (!opts && el.open) el.close();
  }, [opts]);

  return (
    <Ctx.Provider value={ask}>
      {children}
      <dialog
        ref={ref}
        className="confirm"
        aria-labelledby="confirm-title"
        onCancel={(e) => {
          e.preventDefault();
          answer(false);
        }}
        onClick={(e) => {
          if (e.target === ref.current) answer(false);
        }}
      >
        {opts && (
          <div className="confirm__box">
            <h2 id="confirm-title" className="confirm__title">
              {opts.title}
            </h2>
            {opts.text && <p className="confirm__text">{opts.text}</p>}
            <div className="confirm__actions">
              <button type="button" className="btn btn-soft" onClick={() => answer(false)} autoFocus>
                {opts.cancel ?? t("cancel")}
              </button>
              <button type="button" className={"btn " + (opts.danger ? "btn-danger" : "btn-primary")} onClick={() => answer(true)}>
                {opts.ok ?? t("ok")}
              </button>
            </div>
          </div>
        )}
      </dialog>
    </Ctx.Provider>
  );
}
