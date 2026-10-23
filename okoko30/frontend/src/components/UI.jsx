import React from "react";
import {
  Activity,
  ArrowUpRight,
  Check,
  ChevronRight,
  CircleHelp,
  Cloud,
  Database,
  Droplets,
  Flame,
  Gauge,
  House,
  Layers3,
  Leaf,
  Moon,
  Package,
  Radio,
  Sun,
  Thermometer,
  WifiOff,
  Wind,
} from "lucide-react";
import { value } from "../api";
export const icons = {
  boiler: Flame,
  hk1: House,
  hk2: Layers3,
  ecs: Droplets,
  pellets: Package,
  temperatures: Thermometer,
  modulation: Gauge,
  activity: Activity,
  weather: Cloud,
  system: Database,
};
export function Icon({ name, size = 18, ...props }) {
  const Component = icons[name] || CircleHelp;
  return <Component size={size} strokeWidth={1.7} {...props} />;
}
export function Badge({ children, tone = "muted", dot = false }) {
  return (
    <span className={`badge ${tone}`}>
      {dot && <i />}
      {children}
    </span>
  );
}
export function Empty({
  title = "En attente de données",
  children,
  icon: Component = Radio,
  compact = false,
}) {
  return (
    <div className={`empty ${compact ? "compact" : ""}`}>
      <span className="empty-icon">
        <Component size={24} strokeWidth={1.4} />
      </span>
      <strong>{title}</strong>
      <p>
        {children || "Les mesures apparaîtront après une collecte réussie."}
      </p>
    </div>
  );
}
export function Card({
  title,
  eyebrow,
  icon,
  children,
  className = "",
  action,
}) {
  return (
    <section className={`panel ${className}`}>
      <div className="panel-heading">
        <div>
          {eyebrow && <span className="eyebrow">{eyebrow}</span>}
          <h2>
            {icon && <Icon name={icon} />}
            {title}
          </h2>
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}
export function Metric({ label, number, unit, accent = "" }) {
  return (
    <div className={`metric ${accent}`}>
      <span>{label}</span>
      <div>
        {value(number)}
        <small>{unit}</small>
      </div>
    </div>
  );
}
export function KeyValue({ label, children }) {
  return (
    <div className="key-value">
      <span>{label}</span>
      <strong>{children ?? "—"}</strong>
    </div>
  );
}
export function Pump({ on }) {
  return (
    <span className={`pump ${on === true ? "on" : ""}`}>
      <span className="pump-icon">
        <Wind size={13} />
      </span>
      {on === null || on === undefined
        ? "Pompe inconnue"
        : on
          ? "Pompe en marche"
          : "Pompe à l’arrêt"}
    </span>
  );
}
export function StatusText({ text }) {
  return (
    <div className="status-text">
      {text ? (
        text.split("|").map((part, i) => <span key={i}>{part}</span>)
      ) : (
        <span>État non disponible</span>
      )}
    </div>
  );
}
export function PageHeading({ eyebrow, title, description, children }) {
  return (
    <div className="page-heading">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      <div className="heading-actions">{children}</div>
    </div>
  );
}
export function ErrorNotice({ error, retry }) {
  if (!error) return null;
  return (
    <div className="error-notice" role="alert">
      <WifiOff size={18} />
      <span>{error}</span>
      {retry && <button onClick={retry}>Réessayer</button>}
    </div>
  );
}
export function WeatherIcon({ condition, size = 40 }) {
  const c = (condition || "").toLowerCase();
  const Component = /dégagé/.test(c)
    ? Sun
    : /nuag|couvert|pluie|neige/.test(c)
      ? Cloud
      : CircleHelp;
  return <Component size={size} strokeWidth={1.3} className="weather-icon" />;
}
export function BoilerVisual({ temperature, active = false }) {
  return (
    <div
      className={`boiler-visual ${active ? "active" : ""}`}
      aria-hidden="true"
    >
      <svg viewBox="0 0 250 230" fill="none">
        <defs>
          <linearGradient
            id="boilerBody"
            x1="60"
            y1="30"
            x2="180"
            y2="205"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="#273642" />
            <stop offset="1" stopColor="#101c27" />
          </linearGradient>
          <linearGradient id="boilerFace" x1="90" y1="50" x2="175" y2="190">
            <stop stopColor="#e1e5e3" />
            <stop offset="1" stopColor="#81938f" />
          </linearGradient>
          <radialGradient id="flameGlow">
            <stop stopColor="#ff8b47" stopOpacity=".3" />
            <stop offset="1" stopColor="#ff8b47" stopOpacity="0" />
          </radialGradient>
        </defs>
        <ellipse cx="132" cy="211" rx="90" ry="12" fill="#000" opacity=".18" />
        <path
          d="M67 65V24h17v39M172 68V13h15v54"
          stroke="#55656c"
          strokeWidth="9"
        />
        <path
          d="M60 62l28-17h91l20 19v125l-20 15H81l-21-16V62Z"
          fill="url(#boilerBody)"
          stroke="#3d5059"
        />
        <path d="M87 50h90v150H87z" fill="url(#boilerFace)" />
        <path d="M87 50h11v150H87z" fill="#d7deda" opacity=".5" />
        <rect x="112" y="63" width="47" height="25" rx="4" fill="#152631" />
        <rect
          x="119"
          y="69"
          width="25"
          height="10"
          rx="2"
          fill="#6ba7a4"
          opacity=".75"
        />
        <circle cx="151" cy="76" r="2" fill="#ffa76a" />
        <path d="M87 102h89" stroke="#6f807b" />
        <rect
          x="107"
          y="113"
          width="55"
          height="59"
          rx="4"
          fill="#29372f"
          stroke="#5f7166"
        />
        <rect x="112" y="118" width="45" height="49" rx="3" fill="#12231f" />
        <circle cx="134" cy="142" r="35" fill="url(#flameGlow)" />
        <path
          className="flame-path"
          d="M132 126c0 7-9 10-9 17a12 12 0 0 0 23 0c0-4-2-7-4-10 0 5-2 6-4 7 1-6-1-10-6-14Z"
          fill={active ? "#f69c59" : "#55685d"}
        />
        <path
          d="M118 184h30"
          stroke="#52745d"
          strokeWidth="3"
          strokeLinecap="round"
        />
        <text
          x="112"
          y="194"
          fill="#53695c"
          fontSize="6"
          fontFamily="sans-serif"
        >
          ÖkoFEN
        </text>
        <path
          d="M62 187v14h17v-9M177 199v10h17v-19"
          stroke="#3e4e53"
          strokeWidth="5"
        />
        <path
          d="M31 98h28M200 139h28"
          stroke="#3a746e"
          strokeWidth="3"
          strokeDasharray="4 5"
        />
        <circle cx="25" cy="98" r="5" stroke="#4c9d90" />
        <circle cx="233" cy="139" r="5" stroke="#4c9d90" />
      </svg>
      <span className="visual-caption">PELLEMATIC PE CONDENS</span>
    </div>
  );
}
export function SavedIndicator({ error }) {
  return (
    <span className={`saved ${error ? "warning" : ""}`} role="status">
      {error ? <CircleHelp size={14} /> : <Check size={14} />}{" "}
      {error || "Disposition enregistrée"}
    </span>
  );
}
