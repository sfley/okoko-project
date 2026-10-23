import { parseForecastDate, forecastNumber, windLabel } from "../weather-data";
import React from "react";
import {
  ArrowDown,
  ArrowUpRight,
  Check,
  ChevronRight,
  Cloud,
  Clock3,
  Flame,
  Leaf,
  Radio,
  Sun,
  Thermometer,
  Wind,
} from "lucide-react";
import {
  Badge,
  BoilerVisual,
  Empty,
  KeyValue,
  Metric,
  Pump,
  StatusText,
  WeatherIcon,
} from "./UI";
import { CircuitSparkline, HistoryChart } from "./Charts";
import { dateLabel, value } from "../api";
export function BoilerWidget({ snapshot: s }) {
  const b = s?.boiler || {};
  return (
    <div className="boiler-widget">
      <div className="boiler-info">
        <Badge tone={s?.connected ? "green" : "muted"} dot>
          {s?.connected ? "Collecte active" : "En attente"}
        </Badge>
        <div className="boiler-reading">
          {value(b.temperature)}
          <span>°C</span>
        </div>
        <StatusText text={b.status} />
        <div className="boiler-bottom">
          <div>
            <span>Modulation</span>
            <strong>
              {value(b.modulation, 0)} <small>%</small>
            </strong>
          </div>
          <div>
            <span>Foyer</span>
            <strong>
              {value(b.flame, 0)} <small>°C</small>
            </strong>
          </div>
        </div>
      </div>
      <BoilerVisual active={s?.connected && b.modulation > 0} />
    </div>
  );
}
export function CircuitWidget({ snapshot, circuit, detailed = false, history = [] }) {
  const c = snapshot?.circuits?.[circuit] || {};
  return (
    <div className="circuit-widget">
      <div className="circuit-reading">
        <div className="big-reading">
          {value(c.temperature)}
          <small>°C</small>
        </div>
        <span className={`circuit-symbol ${circuit}`}>
          <Thermometer size={28} strokeWidth={1.2} />
        </span>
      </div>
      <div className="setpoint">
        <span>
          {circuit === "ww1" ? "Minimum programmé" : "Consigne programmée"}
        </span>
        <strong>{c.setpoint || "—"}</strong>
      </div>
      {circuit !== "ww1" && <CircuitSparkline data={history} circuit={circuit} name={c.name} />}
      <div className="circuit-footer">
        <Pump on={c.pump} />
        <span>{c.status?.split("|")[0] || "Mode inconnu"}</span>
      </div>
      {detailed && (
        <div className="detail-rows">
          <StatusText text={c.status} />
          <KeyValue label="Départ du circuit">{value(c.flow)} °C</KeyValue>
          {circuit === "ww1" && (
            <KeyValue label="Maximum programmé">{value(c.maximum)} °C</KeyValue>
          )}
        </div>
      )}
    </div>
  );
}
export function PelletGauge({ stock, capacity }) {
  const percent =
    typeof stock === "number" && typeof capacity === "number" && capacity > 0
      ? Math.min(100, Math.max(0, (stock / capacity) * 100))
      : null;
  return (
    <div className="pellet-gauge">
      <svg
        viewBox="0 0 230 145"
        aria-label={
          percent === null
            ? "Niveau inconnu"
            : `Niveau ${Math.round(percent)} pour cent`
        }
        role="img"
      >
        <path
          d="M26 119a89 89 0 0 1 178 0"
          fill="none"
          stroke="#23323b"
          strokeWidth="14"
          strokeLinecap="round"
        />
        {percent !== null && (
          <path
            d="M26 119a89 89 0 0 1 178 0"
            pathLength="100"
            fill="none"
            stroke={percent < 20 ? "#e78076" : "#61bbb0"}
            strokeWidth="14"
            strokeLinecap="round"
            strokeDasharray={`${percent} 100`}
          />
        )}
        <text
          x="115"
          y="102"
          textAnchor="middle"
          fill="#edf1f2"
          fontSize="32"
          fontWeight="500"
        >
          {value(stock, 0)}
        </text>
        <text x="115" y="123" textAnchor="middle" fill="#90a4af" fontSize="11">
          kg disponibles
        </text>
      </svg>
      <span className="gauge-caption">
        {percent === null
          ? "Capacité non renseignée"
          : `${Math.round(percent)} % du silo · ${value(capacity, 0)} kg`}
      </span>
    </div>
  );
}
export function PelletsWidget({ snapshot }) {
  const p = snapshot?.pellets || {};
  return (
    <div className="pellets-widget">
      <PelletGauge stock={p.stock} capacity={p.capacity} />
      <div className="pellet-today">
        <span>
          <Leaf size={15} /> Aujourd’hui
        </span>
        <strong>
          {value(p.today)} <small>kg</small>
        </strong>
      </div>
    </div>
  );
}
export function WeatherWidget({ weather, snapshot }) {
  const today = parseForecastDate(weather?.current_day)?.date || new Date();
  const f = weather?.forecast?.find((item) => {
    const parsed = parseForecastDate(item.date);
    return parsed && parsed.date.toDateString() === today.toDateString();
  });
  const n = forecastNumber(f?.temp);
  return (
    <div className="weather-widget">
      <div className="weather-now">
        <WeatherIcon condition={f?.condition} size={55} />
        <div className="big-reading">
          {value(n !== null ? n : snapshot?.outdoor)}
          <small>°C</small>
        </div>
      </div>
      <strong>{f?.condition || "Météo non disponible"}</strong>
      <span>Prévisions de votre chaudière</span>
      <div className="weather-meta">
        <span>
          <Wind size={14} /> {windLabel(f?.wind)}
        </span>
        <span>
          <Sun size={14} /> {f && f.is_current !== false ? weather?.current?.Lever || "—" : "—"}
        </span>
      </div>
    </div>
  );
}
export function ActivityWidget({ transitions = [], snapshot }) {
  if (!transitions.length)
    return (
      <Empty compact title="Une maison sereine">
        Les changements d’état observés seront regroupés ici.
      </Empty>
    );
  return (
    <div className="activity-list">
      {transitions.slice(0, 4).map((t, i) => (
        <div className="activity-item" key={`${t.ts}-${i}`}>
          <span className={`activity-dot ${t.circuit}`} />
          <div>
            <strong>
              {snapshot?.circuits?.[t.circuit]?.name ||
                {
                  pe1: "Chaudière",
                  hk1: "Circuit HK1",
                  hk2: "Circuit HK2",
                  ww1: "Eau chaude",
                }[t.circuit]
              }
            </strong>
            <p>
              {t.kind === "pump"
                ? `Pompe ${t.after === "1" ? "en marche" : "à l’arrêt"}`
                : t.after}
            </p>
          </div>
          <time>{dateLabel(t.ts, { hour: "2-digit", minute: "2-digit" })}</time>
        </div>
      ))}
    </div>
  );
}
export function SystemWidget({ snapshot }) {
  return (
    <div className="system-widget">
      <div className="system-orbit">
        <Radio size={29} />
      </div>
      <strong>
        {snapshot?.connected ? "À votre écoute" : "Prêt à se connecter"}
      </strong>
      <p>
        {snapshot?.connected
          ? "Collecte JSON active"
          : "Configurez la connexion chaudière"}
      </p>
      <KeyValue label="Intervalle de collecte">
        {snapshot?.poll_seconds || "—"} s
      </KeyValue>
      <KeyValue label="Hébergement">Local · Go / SQLite</KeyValue>
    </div>
  );
}
export function ModulationWidget({ snapshot, history = [] }) {
  return (
    <div className="modulation-widget">
      <Metric
        label="Puissance actuelle"
        number={snapshot?.boiler?.modulation}
        unit="%"
        accent="orange"
      />
      <HistoryChart compact data={history} keys={["modulation"]} unit="%" />
    </div>
  );
}
