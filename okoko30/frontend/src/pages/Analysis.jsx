import React, { useState } from "react";
import { Download, SlidersHorizontal } from "lucide-react";
import { dateLabel, useResource } from "../api";
import { Badge, Card, Empty, ErrorNotice, PageHeading } from "../components/UI";
import { HistoryChart } from "../components/Charts";
const defaultCircuitNames = {
  pe1: "Chaudière",
  hk1: "HK1",
  hk2: "HK2",
  ww1: "Eau chaude",
};
export default function Analysis({ snapshot }) {
  const circuitNames = { ...defaultCircuitNames, ...Object.fromEntries(Object.entries(snapshot?.circuits || {}).filter(([, c]) => c.name).map(([id, c]) => [id, c.name])) };
  const [days, setDays] = useState(1);
  const [circuit, setCircuit] = useState("all");
  const history = useResource(`history?days=${days}`, 60000);
  const transitions = useResource("transitions", 60000);
  const visible = (transitions.data || []).filter(
    (t) => circuit === "all" || t.circuit === circuit,
  );
  return (
    <>
      <PageHeading
        eyebrow="Comprendre pour mieux vivre"
        title={
          <>
            Le recul fait <span>la différence.</span>
          </>
        }
        description="Températures, puissance et changements d’état : suivez le fil."
      >
        <a className="button" href="/api/v3/export?type=measurements">
          <Download size={16} /> Exporter les mesures
        </a>
      </PageHeading>
      <ErrorNotice error={history.error} retry={history.reload} />
      <Card
        title="L’équilibre thermique"
        eyebrow="Historique des températures"
        icon="temperatures"
        action={
          <div className="segmented" aria-label="Période">
            {[
              [1, "24 h"],
              [7, "7 jours"],
              [30, "30 jours"],
            ].map(([n, label]) => (
              <button
                key={n}
                aria-pressed={days === n}
                onClick={() => setDays(n)}
              >
                {label}
              </button>
            ))}
          </div>
        }
      >
        <HistoryChart data={history.data || []} snapshot={snapshot} />
        <div className="chart-footnote">
          Source : imports CSV et journaux de la chaudière · Les valeurs
          absentes restent absentes.
        </div>
      </Card>
      <div className="two-columns analysis-secondary">
        <Card
          title="Modulation"
          icon="modulation"
          eyebrow="Le rythme de la chaudière"
        >
          <HistoryChart
            data={history.data || []}
            keys={["modulation"]}
            compact
            unit="%"
          />
        </Card>
        <Card
          title="Évolution du silo"
          icon="pellets"
          eyebrow="Stock estimé par la chaudière"
        >
          <HistoryChart
            data={history.data || []}
            keys={["stock"]}
            compact
            unit="kg"
          />
        </Card>
      </div>
      <Card
        title="Le journal de votre installation"
        eyebrow="Transitions observées"
        icon="activity"
        action={
          <select
            aria-label="Filtrer les transitions"
            value={circuit}
            onChange={(e) => setCircuit(e.target.value)}
          >
            <option value="all">Tous les circuits</option>
            {Object.entries(circuitNames).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </select>
        }
      >
        <ErrorNotice error={transitions.error} retry={transitions.reload} />
        {visible.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Observation</th>
                  <th>Circuit</th>
                  <th>Événement</th>
                  <th>État précédent</th>
                  <th>Nouvel état</th>
                </tr>
              </thead>
              <tbody>
                {visible.map((t, i) => (
                  <tr key={i}>
                    <td>
                      {dateLabel(t.ts, {
                        day: "2-digit",
                        month: "short",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </td>
                    <td>
                      <Badge tone="teal">{circuitNames[t.circuit]}</Badge>
                    </td>
                    <td>{t.kind === "pump" ? "Pompe" : "Mode / état"}</td>
                    <td className="muted">{t.before}</td>
                    <td>{t.after}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="Pas encore de transition">
            Les changements sont enregistrés lors des collectes. Leur heure
            indique l’observation, pas nécessairement le changement exact.
          </Empty>
        )}
        <div className="chart-footnote">
          100 dernières observations. Aucun changement n’est reconstitué entre
          deux collectes.
        </div>
      </Card>
    </>
  );
}
