// Иконки кухонной техники: одна линия 1.8px, viewBox 32×32, как у lucide.
// У каждой — маленькая часть, которая оживает, когда прибор выбран (класс .eq--on):
// пар над кастрюлей, вентилятор аэрогриля, волны микроволновки, нож блендера.

type Props = { id: string; on?: boolean; size?: number };

const common = { fill: "none", stroke: "currentColor", strokeWidth: 1.8, strokeLinecap: "round" as const, strokeLinejoin: "round" as const };

export function EquipmentIcon({ id, on = false, size = 24 }: Props) {
  const cls = "eq" + (on ? " eq--on" : "");
  const svg = (children: React.ReactNode) => (
    <svg className={cls} width={size} height={size} viewBox="0 0 32 32" aria-hidden="true" focusable="false" {...common}>
      {children}
    </svg>
  );
  switch (id) {
    case "stove":
      return svg(
        <>
          <path d="M6 14h20v12a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2z" />
          <path d="M4 14h24" />
          <circle cx="11" cy="20" r="2.4" />
          <circle cx="21" cy="20" r="2.4" />
          <g className="eq__steam">
            <path d="M12 9c0-2 2-2 2-4" />
            <path d="M17 9c0-2 2-2 2-4" />
          </g>
        </>,
      );
    case "oven":
      return svg(
        <>
          <rect x="5" y="6" width="22" height="20" rx="2" />
          <path d="M5 12h22" />
          <rect x="9" y="15" width="14" height="7" rx="1" className="eq__glow" />
          <circle cx="9" cy="9" r="0.9" fill="currentColor" stroke="none" />
          <circle cx="13" cy="9" r="0.9" fill="currentColor" stroke="none" />
        </>,
      );
    case "microwave":
      return svg(
        <>
          <rect x="4" y="8" width="24" height="16" rx="2" />
          <rect x="7" y="11" width="13" height="10" rx="1" />
          <path d="M23 12v1M23 16v1" />
          <g className="eq__waves">
            <path d="M10 16c1-1.5 2-1.5 3 0s2 1.5 3 0" />
          </g>
        </>,
      );
    case "airfryer":
      return svg(
        <>
          <path d="M9 6h14l2 6v12a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2V12z" />
          <path d="M7 12h18" />
          <g className="eq__fan" style={{ transformOrigin: "16px 19px" }}>
            <path d="M16 15.5v7M12.5 19h7" />
          </g>
          <circle cx="16" cy="19" r="4.5" />
        </>,
      );
    case "multicooker":
      // Приземистый корпус, выпуклая крышка с петлёй сзади и кнопкой спереди, панель с окошком.
      return svg(
        <>
          <path d="M6 13h20v10a3 3 0 0 1-3 3H9a3 3 0 0 1-3-3z" />
          <path d="M7 13c0-4 4-6 9-6s9 2 9 6" />
          <path d="M14 7.5V6h4v1.5" />
          <path d="M24 12l2 1" />
          <rect x="11" y="17" width="10" height="4" rx="1" className="eq__glow" />
          <path d="M4 26h24" />
          <g className="eq__steam">
            <path d="M16 5c0-1.5 1-1.5 1-3" />
          </g>
        </>,
      );
    case "blender":
      return svg(
        <>
          <path d="M11 6h10l-1.5 14h-7z" />
          <path d="M9 20h14v3a2 2 0 0 1-2 2H11a2 2 0 0 1-2-2z" />
          <path d="M13 26v2M19 26v2" />
          <g className="eq__blade" style={{ transformOrigin: "16px 13px" }}>
            <path d="M13 13h6M16 10v6" />
          </g>
        </>,
      );
    case "mixer":
      return svg(
        <>
          <path d="M8 8h13a4 4 0 0 1 0 8H8z" />
          <path d="M12 16v8M20 16v8" />
          <g className="eq__whisk" style={{ transformOrigin: "12px 24px" }}>
            <path d="M10 24l2 4 2-4" />
          </g>
          <g className="eq__whisk" style={{ transformOrigin: "20px 24px" }}>
            <path d="M18 24l2 4 2-4" />
          </g>
        </>,
      );
    case "grill":
      return svg(
        <>
          <path d="M6 12h20l-2 8H8z" />
          <path d="M9 20l-2 6M23 20l2 6M16 20v6" />
          <path d="M10 16h12" />
          <g className="eq__steam">
            <path d="M12 9c0-2 2-2 2-4" />
            <path d="M18 9c0-2 2-2 2-4" />
          </g>
        </>,
      );
    case "steamer":
      return svg(
        <>
          <path d="M7 16h18v8a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2z" />
          <path d="M9 16v-4h14v4" />
          <path d="M12 14h1M15.5 14h1M19 14h1" />
          <g className="eq__steam">
            <path d="M13 9c0-2 2-2 2-4" />
            <path d="M18 9c0-2 2-2 2-4" />
          </g>
        </>,
      );
    case "meatgrinder":
      return svg(
        <>
          <path d="M8 14h12v8H8z" />
          <path d="M12 14V9h4v5" />
          <path d="M20 17h5" />
          <g className="eq__crank" style={{ transformOrigin: "25px 17px" }}>
            <path d="M25 17l3-3" />
          </g>
          <path d="M9 22v4M19 22v4" />
        </>,
      );
    default:
      return svg(<rect x="6" y="6" width="20" height="20" rx="4" />);
  }
}
