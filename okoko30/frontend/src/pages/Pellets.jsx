import React, { useEffect, useState } from "react";
import { Check, Download, Leaf, Plus, X } from "lucide-react";
import { dateLabel, request, useResource, value } from "../api";
import {
  Badge,
  Card,
  Empty,
  ErrorNotice,
  KeyValue,
  Metric,
  PageHeading,
} from "../components/UI";
import { MonthlyChart } from "../components/Charts";
import { PelletGauge } from "../components/Widgets";
export default function Pellets({ snapshot }) {
  const resource = useResource("pellets");
  const [year, setYear] = useState(new Date().getFullYear());
  const [modal, setModal] = useState(false);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const data = resource.data || { monthly: [], refills: [] };
  const years = [
    ...new Set([new Date().getFullYear(), ...data.monthly.map((r) => r.year)]),
  ].sort((a, b) => b - a);
  const annual = data.monthly.filter((r) => r.year === Number(year));
  const total = annual.length ? annual.reduce((sum, r) => sum + r.kg, 0) : null;
  const pellets = snapshot?.pellets || {};
  async function submit(event) {
    event.preventDefault();
    setError("");
    setSaving(true);
    const f = new FormData(event.currentTarget);
    try {
      await request("refills", {
        method: "POST",
        body: JSON.stringify({
          date: f.get("date"),
          kg: Number(f.get("kg")),
          supplier: f.get("supplier"),
          kind: f.get("kind"),
          note: f.get("note"),
        }),
      });
      setModal(false);
      setSaved(true);
      resource.reload();
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <>
      <PageHeading
        eyebrow="Une énergie qui se suit"
        title={
          <>
            Chaque pellet <span>compte.</span>
          </>
        }
        description="Votre réserve, vos consommations et vos recharges, au même endroit."
      >
        <button
          className="button primary"
          onClick={() => {
            setModal(true);
            setError("");
          }}
        >
          <Plus size={17} /> Ajouter une recharge
        </button>
      </PageHeading>
      <ErrorNotice error={resource.error} retry={resource.reload} />
      {saved && (
        <div className="success-notice" role="status">
          <Check size={17} /> Recharge enregistrée. L’historique a été
          actualisé.
          <button
            className="icon-button"
            onClick={() => setSaved(false)}
            aria-label="Fermer"
          >
            <X size={15} />
          </button>
        </div>
      )}
      <div className="pellets-overview">
        <Card
          title="Votre réserve"
          eyebrow="Silo à pellets"
          icon="pellets"
          className="reserve-card"
        >
          <PelletGauge stock={pellets.stock} capacity={pellets.capacity} />
          <span className="muted">Stock estimé par la chaudière</span>
        </Card>
        <Card title="Au fil des jours" icon="pellets">
          <div className="metrics-pair">
            <Metric
              label="Aujourd’hui"
              number={pellets.today}
              unit="kg"
              accent="teal"
            />
            <Metric label="Hier" number={pellets.yesterday} unit="kg" />
          </div>
          <KeyValue label={`Bilan ${year}`}>{value(total, 0)} kg</KeyValue>
          <KeyValue label="Mois renseignés">{annual.length} / 12</KeyValue>
          <p className="small-note">
            Les mois absents ne sont pas assimilés à une consommation nulle.
          </p>
        </Card>
        <Card title="Dernière recharge" icon="activity">
          {data.refills.length ? (
            <>
              <div className="refill-highlight">
                <span>{value(data.refills[0].kg, 0)}</span> kg
              </div>
              <KeyValue label="Date">
                {dateLabel(data.refills[0].date, {
                  day: "2-digit",
                  month: "long",
                  year: "numeric",
                })}
              </KeyValue>
              <KeyValue label="Fournisseur">
                {data.refills[0].supplier || "Non renseigné"}
              </KeyValue>
              <KeyValue label="Conditionnement">
                {data.refills[0].kind}
              </KeyValue>
            </>
          ) : (
            <Empty compact title="Aucune recharge" />
          )}
        </Card>
      </div>
      <Card
        title="Votre consommation, mois après mois"
        eyebrow="Bilan annuel"
        icon="modulation"
        action={
          <select
            aria-label="Année du bilan"
            value={year}
            onChange={(e) => setYear(Number(e.target.value))}
          >
            {years.map((y) => (
              <option key={y}>{y}</option>
            ))}
          </select>
        }
      >
        <MonthlyChart rows={data.monthly} year={year} />
        <div className="annual-summary">
          <span>
            {annual.length} mois renseignés en {year}
          </span>
          <strong>
            {value(total, 0)} <small>kg au total</small>
          </strong>
          <a className="text-link" href="/api/v3/export?type=monthly">
            <Download size={14} /> Exporter le bilan
          </a>
        </div>
      </Card>
      <Card
        title="Les recharges de votre silo"
        eyebrow="Historique unique"
        icon="pellets"
        action={<Badge>{data.refills.length} recharges</Badge>}
      >
        {data.refills.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Quantité</th>
                  <th>Fournisseur</th>
                  <th>Format</th>
                  <th>Note</th>
                </tr>
              </thead>
              <tbody>
                {data.refills.map((r, i) => (
                  <tr key={`${r.date}-${i}`}>
                    <td>
                      {dateLabel(r.date, {
                        day: "2-digit",
                        month: "short",
                        year: "numeric",
                      })}
                    </td>
                    <td className="quantity">
                      + {value(r.kg, 0)} <small>kg</small>
                    </td>
                    <td>{r.supplier || "—"}</td>
                    <td>
                      <Badge>{r.kind}</Badge>
                    </td>
                    <td className="muted">{r.note || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="Votre historique commence ici" />
        )}
      </Card>
      <div className="info-banner">
        <Leaf size={20} />
        <div>
          <strong>Vos repères restent les mêmes</strong>
          <p>
            Calcul mensuel conservé : premier stock − dernier stock + recharges
            manuelles. Les archives et corrections de la version précédente
            restent préservées.
          </p>
        </div>
      </div>
      {modal && (
        <div className="modal-backdrop">
          <section
            className="modal refill-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="refill-title"
          >
            <div className="modal-heading">
              <div>
                <span className="eyebrow">Le plein de chaleur</span>
                <h2 id="refill-title">Ajouter une recharge</h2>
              </div>
              <button
                className="icon-button"
                aria-label="Fermer"
                onClick={() => setModal(false)}
              >
                <X size={20} />
              </button>
            </div>
            <form onSubmit={submit}>
              <div className="form-grid">
                <label>
                  Date de livraison
                  <input
                    autoFocus
                    type="date"
                    name="date"
                    required
                    defaultValue={new Date().toLocaleDateString("en-CA")}
                  />
                </label>
                <label>
                  Quantité en kg
                  <input
                    type="number"
                    name="kg"
                    min="0.01"
                    max="10000"
                    step="0.01"
                    required
                    placeholder="Ex. 2 000"
                  />
                </label>
                <label>
                  Fournisseur
                  <input
                    name="supplier"
                    maxLength={120}
                    placeholder="Facultatif"
                  />
                </label>
                <label>
                  Conditionnement
                  <select name="kind">
                    <option>Vrac</option>
                    <option>Sacs</option>
                  </select>
                </label>
                <label className="full">
                  Note
                  <input
                    name="note"
                    maxLength={500}
                    placeholder="Une précision à conserver…"
                  />
                </label>
              </div>
              <ErrorNotice error={error} />
              <div className="modal-footer">
                <button
                  type="button"
                  className="button"
                  onClick={() => setModal(false)}
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  className="button primary"
                  disabled={saving}
                >
                  {saving ? "Enregistrement…" : "Enregistrer la recharge"}
                </button>
              </div>
            </form>
          </section>
        </div>
      )}
    </>
  );
}
