import React, { useId, useState } from "react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Empty } from "./UI";
import { dateLabel, value } from "../api";
const colors = {
  hk1: "#5bbcb4",
  hk2: "#8e9af1",
  ww1: "#f6a665",
  outdoor: "#b2c9da",
  modulation: "#f6a665",
  stock: "#5bbcb4",
};
const names = {
  hk1: "HK1",
  hk2: "HK2",
  ww1: "Eau chaude",
  outdoor: "Extérieur",
  modulation: "Modulation",
  stock: "Stock",
};
function ChartTooltip({ active, payload, label, unit = "°C" }) {
  if (!active || !payload?.length) return null;
  return (
    <div className="chart-tooltip">
      <strong>
        {dateLabel(label, {
          day: "2-digit",
          month: "short",
          hour: "2-digit",
          minute: "2-digit",
        })}
      </strong>
      {payload.map((p) => (
        <div key={p.dataKey}>
          <i style={{ background: p.color }} />
          <span>{p.name}</span>
          <b>
            {value(p.value)} {unit}
          </b>
        </div>
      ))}
    </div>
  );
}
export function HistoryChart({
  data = [],
  keys = ["hk1", "hk2", "ww1", "outdoor"],
  snapshot,
  compact = false,
  unit = "°C",
}) {
  const [hidden, setHidden] = useState([]);
  const id = useId().replaceAll(":", "");
  const labels = { ...names, ...Object.fromEntries(Object.entries(snapshot?.circuits || {}).filter(([, c]) => c.name).map(([id, c]) => [id, c.name])) };
  if (!data.some((row) => keys.some((k) => typeof row[k] === "number")))
    return (
      <Empty compact={compact} title="L’historique prend forme ici">
        Les mesures CSV importées alimenteront ce graphique.
      </Empty>
    );
  return (
    <div className={`history-chart ${compact ? "compact-chart" : ""}`}>
      <div className="chart-legend">
        {keys.map((k) => (
          <button
            key={k}
            aria-pressed={!hidden.includes(k)}
            onClick={() =>
              setHidden((h) =>
                h.includes(k) ? h.filter((x) => x !== k) : [...h, k],
              )
            }
          >
            <i style={{ background: colors[k] }} />
            {labels[k]}
          </button>
        ))}
        <span>{unit}</span>
      </div>
      <div className="chart-canvas">
        <ResponsiveContainer width="100%" height="100%" minWidth={0}>
          <AreaChart
            data={data}
            margin={{ left: -22, right: 10, top: 12, bottom: 0 }}
          >
            <defs>
              {keys.map((k) => (
                <linearGradient
                  key={k}
                  id={`${id}-${k}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="0%" stopColor={colors[k]} stopOpacity={0.13} />
                  <stop offset="100%" stopColor={colors[k]} stopOpacity={0} />
                </linearGradient>
              ))}
            </defs>
            <CartesianGrid
              stroke="#24313d"
              strokeDasharray="3 5"
              vertical={false}
            />
            <XAxis
              dataKey="ts"
              axisLine={false}
              tickLine={false}
              minTickGap={45}
              tick={{ fill: "#8495a5", fontSize: 11 }}
              tickFormatter={(t) =>
                dateLabel(t, { hour: "2-digit", minute: "2-digit" })
              }
            />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{ fill: "#8495a5", fontSize: 11 }}
              domain={["auto", "auto"]}
            />
            <Tooltip
              content={<ChartTooltip unit={unit} />}
              cursor={{ stroke: "#647684", strokeDasharray: "3 3" }}
            />
            {keys.map((k) => (
              <Area
                key={k}
                type="monotone"
                dataKey={k}
                name={labels[k]}
                stroke={colors[k]}
                strokeWidth={2}
                fill={`url(#${id}-${k})`}
                hide={hidden.includes(k)}
                connectNulls={false}
                isAnimationActive={false}
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
export function CircuitSparkline({ data = [], circuit, name }) {
  const hasData = data.some((row) => typeof row[circuit] === "number");
  return (
    <div className="circuit-history" role="group" aria-label={`Historique 24 heures · ${name || circuit.toUpperCase()}`}>
      <div className="circuit-history-label"><span>Température · 24 h</span><span>°C</span></div>
      {hasData ? <div className="circuit-sparkline">
        <ResponsiveContainer width="100%" height="100%" minWidth={0}>
          <LineChart data={data} margin={{ top: 4, right: 2, bottom: 3, left: 2 }}>
            <XAxis dataKey="ts" hide />
            <YAxis hide domain={["dataMin - 0.3", "dataMax + 0.3"]} />
            <Tooltip content={<ChartTooltip />} cursor={{ stroke: "#647684", strokeDasharray: "3 3" }} />
            <Line type="monotone" dataKey={circuit} name={name || names[circuit]} stroke={colors[circuit]} strokeWidth={2} dot={false} activeDot={{ r: 3 }} connectNulls={false} isAnimationActive={false} />
          </LineChart>
        </ResponsiveContainer>
      </div> : <div className="circuit-history-empty">Historique CSV indisponible</div>}
    </div>
  );
}
export function MonthlyChart({ rows = [], year }) {
  const months = [
    "Jan.",
    "Fév.",
    "Mars",
    "Avr.",
    "Mai",
    "Juin",
    "Juil.",
    "Août",
    "Sept.",
    "Oct.",
    "Nov.",
    "Déc.",
  ];
  const data = months.map((month, i) => ({
    month,
    kg:
      rows.find((r) => r.year === Number(year) && r.month === i + 1)?.kg ??
      null,
  }));
  if (!data.some((x) => x.kg !== null))
    return (
      <Empty title="Aucun bilan pour cette année">
        Sélectionnez une année disposant de mesures ou d’archives.
      </Empty>
    );
  return (
    <div className="monthly-chart">
      <ResponsiveContainer width="100%" height="100%" minWidth={0}>
        <BarChart
          data={data}
          margin={{ left: -15, top: 20, right: 12, bottom: 0 }}
        >
          <CartesianGrid
            stroke="#24313d"
            strokeDasharray="3 5"
            vertical={false}
          />
          <XAxis
            dataKey="month"
            tick={{ fill: "#94a4b3", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis
            tick={{ fill: "#8495a5", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
          />
          <Tooltip
            cursor={{ fill: "#ffffff05" }}
            contentStyle={{
              background: "#172430",
              border: "1px solid #344452",
              borderRadius: 12,
            }}
            formatter={(v) => [`${value(v)} kg`, "Consommation"]}
          />
          <Bar
            dataKey="kg"
            fill="#d9915f"
            radius={[5, 5, 0, 0]}
            maxBarSize={38}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
