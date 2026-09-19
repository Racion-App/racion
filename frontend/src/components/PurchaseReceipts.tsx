import { useMemo, useState } from "react";
import { ChevronDown, ShoppingBag } from "lucide-react";
import type { Purchase } from "../lib/types";
import { dateShort, monthLabel } from "../lib/format";
import { useT } from "../i18n";

// История покупок как стопка маленьких чеков: день — один чек с датой, числом позиций и суммой,
// свёрнутый показывает первые три строки и «ещё N», разворачивается на месте. Дни сгруппированы
// по месяцам с итогом месяца, так что даже за год страница остаётся обозримой.

const PREVIEW = 3;

export function PurchaseReceipts({ purchases, money }: { purchases: Purchase[]; money: (v: number) => string }) {
  const { t, tn, lang } = useT();
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const [monthsShown, setMonthsShown] = useState(2);

  const months = useMemo(() => {
    const byDay = new Map<string, Purchase[]>();
    for (const p of purchases) {
      const d = p.boughtAt.slice(0, 10);
      byDay.set(d, [...(byDay.get(d) ?? []), p]);
    }
    const byMonth = new Map<string, [string, Purchase[]][]>();
    for (const [day, items] of byDay) {
      const m = day.slice(0, 7);
      byMonth.set(m, [...(byMonth.get(m) ?? []), [day, items]]);
    }
    return [...byMonth.entries()];
  }, [purchases]);

  return (
    <div className="preceipts">
      {months.slice(0, monthsShown).map(([month, days]) => {
        const total = days.reduce((s, [, items]) => s + items.reduce((a, x) => a + x.cost, 0), 0);
        const count = days.reduce((s, [, items]) => s + items.length, 0);
        return (
          <section className="preceipts__month" key={month} aria-label={monthLabel(month, lang)}>
            <header className="preceipts__mhead">
              <h3>{monthLabel(month, lang)}</h3>
              <span className="num">
                {days.length} {tn("trips", days.length)} · {count} {tn("items", count)} · {money(total)}
              </span>
            </header>
            <div className="preceipts__grid">
              {days.map(([day, items]) => {
                const isOpen = !!open[day];
                const shown = isOpen ? items : items.slice(0, PREVIEW);
                const rest = items.length - PREVIEW;
                const sum = items.reduce((s, x) => s + x.cost, 0);
                return (
                  <article className={"preceipt" + (isOpen ? " is-open" : "")} key={day}>
                    <header className="preceipt__head">
                      <span className="preceipt__icon" aria-hidden>
                        <ShoppingBag size={14} />
                      </span>
                      <b>{dateShort(day, lang)}</b>
                      <span className="preceipt__count num">{items.length} {tn("items", items.length)}</span>
                    </header>
                    <ul className="preceipt__lines">
                      {shown.map((p) => (
                        <li key={p.id}>
                          <span className="preceipt__name">{p.name}</span>
                          <span className="preceipt__dots" aria-hidden />
                          <span className="preceipt__qty num">{p.qty}</span>
                          <span className="preceipt__cost num">{p.cost > 0 ? money(p.cost) : "—"}</span>
                        </li>
                      ))}
                    </ul>
                    {rest > 0 && (
                      <button type="button" className="preceipt__more" onClick={() => setOpen({ ...open, [day]: !isOpen })} aria-expanded={isOpen}>
                        <ChevronDown size={14} aria-hidden /> {isOpen ? t("purch.less") : t("purch.more", { n: rest })}
                      </button>
                    )}
                    <footer className="preceipt__total">
                      <span>{t("purch.total")}</span>
                      <b className="num">{money(sum)}</b>
                    </footer>
                  </article>
                );
              })}
            </div>
          </section>
        );
      })}
      {months.length > monthsShown && (
        <button type="button" className="btn btn-soft preceipts__older" onClick={() => setMonthsShown(monthsShown + 2)}>
          {t("purch.older", { n: months.length - monthsShown })}
        </button>
      )}
    </div>
  );
}
