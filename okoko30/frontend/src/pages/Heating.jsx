import React from "react";
import { ArrowUpRight, LockKeyhole, Waves, Wind } from "lucide-react";
import { Badge, Card, KeyValue, Metric, PageHeading } from "../components/UI";
import { BoilerWidget, CircuitWidget } from "../components/Widgets";
import { useResource, value } from "../api";
export default function Heating({ snapshot }) {
  const b = snapshot?.boiler || {};
  const history = useResource("history?days=1", 60000);
  return (
    <>
      <PageHeading
        eyebrow="Votre installation"
        title={
          <>
            La chaleur, <span>au bon endroit.</span>
          </>
        }
        description="Comprenez chaque circuit et retrouvez vos réglages programmés."
      >
        <Badge tone={snapshot?.connected ? "green" : "muted"} dot>
          {snapshot?.connected ? "Données à jour" : "Mesures indisponibles"}
        </Badge>
      </PageHeading>
      <div className="heating-top">
        <Card
          title="Le cœur de votre maison"
          eyebrow="Chaudière ÖkoFEN"
          className="heating-boiler"
        >
          <BoilerWidget snapshot={snapshot} />
        </Card>
        <Card title="En détail" icon="modulation">
          <div className="metrics-pair">
            <Metric label="Ventilation" number={b.airflow} unit="%" />
            <Metric label="Dépression" number={b.pressure} unit="Pa" />
          </div>
          <KeyValue label="Pompe primaire">{value(b.pump, 0)} %</KeyValue>
          <KeyValue label="Fonctionnement cumulé">
            {value(b.runtime, 0)} h
          </KeyValue>
          <KeyValue label="Démarrages">{value(b.starts, 0)}</KeyValue>
          <KeyValue label="Cycle moyen">
            {value(b.average_runtime, 0)} min
          </KeyValue>
        </Card>
      </div>
      <div className="section-title">
        <h2>Vos espaces de confort</h2>
        <span>Modes et consignes issus de la chaudière</span>
      </div>
      <div className="three-columns">
        {[
          ["hk1", "Circuit de chauffage HK1", "hk1"],
          ["hk2", "Circuit de chauffage HK2", "hk2"],
          ["ww1", "Eau chaude sanitaire", "ecs"],
        ].map(([id, title, icon]) => (
          <Card
            key={id}
            title={snapshot?.circuits?.[id]?.name || title}
            eyebrow={id === "ww1" ? "ECS" : id.toUpperCase()}
            icon={icon}
          >
            <CircuitWidget snapshot={snapshot} circuit={id} history={history.data || []} detailed />
          </Card>
        ))}
      </div>
      <div className="info-banner">
        <LockKeyhole size={19} />
        <div>
          <strong>Superviser en toute confiance</strong>
          <p>
            Les consignes affichées proviennent du programme actif. Les
            commandes de chauffe restent sur votre régulation ÖkoFEN.
          </p>
        </div>
      </div>
    </>
  );
}
