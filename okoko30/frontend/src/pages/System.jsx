import React, { useState } from "react";
import {
  Archive,
  ArrowDownToLine,
  Check,
  Database,
  Download,
  HardDrive,
  Radio,
  RefreshCw,
  Server,
  ShieldCheck,
} from "lucide-react";
import { dateLabel, request, useResource, value } from "../api";
import {
  Badge,
  Card,
  Empty,
  ErrorNotice,
  KeyValue,
  PageHeading,
} from "../components/UI";
export default function System({ snapshot }) {
  const [page, setPage] = useState(1);
  const resource = useResource(`system?page=${page}`, 10000);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [started, setStarted] = useState(false);
  const s = resource.data;
  const pagination = s?.pagination;
  async function runImport() {
    setBusy(true);
    setError("");
    try {
      await request("import", { method: "POST", body: "{}" });
      setStarted(true);
      resource.reload();
    } catch (e) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }
  const systems = [
    {
      name: "Collecte chaudière",
      icon: Radio,
      ok: snapshot?.connected,
      configured: snapshot?.configured,
      detail: snapshot?.connected
        ? `Lecture toutes les ${snapshot.poll_seconds} secondes`
        : snapshot?.configured
          ? "Connexion en attente"
          : "Connexion non configurée",
    },
    {
      name: "Base SQLite",
      icon: Database,
      ok: s?.sqlite,
      configured: true,
      detail: s
        ? `${value(s.measurements, 0)} mesures conservées`
        : "Vérification en cours",
    },
    {
      name: "Archives NAS",
      icon: HardDrive,
      ok: s?.nas_mounted,
      configured: s?.nas_configured,
      detail: s?.nas_mounted
        ? "Montage NFS détecté"
        : s?.nas_configured
          ? "NAS indisponible · application autonome"
          : "NAS non configuré · facultatif",
    },
  ];
  return (
    <>
      <PageHeading
        eyebrow="Les coulisses de votre confort"
        title={
          <>
            Tout est <span>sous contrôle.</span>
          </>
        }
        description="Un état clair de votre installation. Des données qui restent chez vous."
      >
        <Badge>OKOKO {snapshot?.version || "3.0.1"}</Badge>
      </PageHeading>
      <ErrorNotice error={resource.error} retry={resource.reload} />
      <ErrorNotice error={error} />
      <div className="three-columns system-status-grid">
        {systems.map(({ name, icon: Icon, ok, configured, detail }) => (
          <section className="panel system-status" key={name}>
            <div>
              <span className={`system-status-icon ${ok ? "ok" : ""}`}>
                <Icon size={22} />
              </span>
              <Badge tone={ok ? "green" : "muted"} dot>
                {ok
                  ? "Opérationnel"
                  : configured
                    ? "En attente"
                    : "Non configuré"}
              </Badge>
            </div>
            <h2>{name}</h2>
            <p>{detail}</p>
          </section>
        ))}
      </div>
      <div className="two-columns system-actions">
        <Card
          title="Vos données méritent de durer"
          eyebrow="Sauvegarde locale"
          icon="system"
        >
          <div className="backup-illustration">
            <Archive size={45} strokeWidth={1.1} />
            <span>
              <ShieldCheck size={22} />
            </span>
          </div>
          <p className="panel-description">
            Téléchargez une copie cohérente de SQLite, y compris les écritures
            en cours. Son intégrité est vérifiée avant l’envoi.
          </p>
          <a className="button primary" href="/api/v3/backup">
            <Download size={16} /> Télécharger la sauvegarde
          </a>
          <p className="small-note">
            Archive ZIP · Aucun mot de passe de configuration inclus
          </p>
        </Card>
        <Card
          title="Des collectes à votre rythme"
          eyebrow="Import JSON / CSV"
          icon="activity"
        >
          <KeyValue label="Lecture JSON">
            {snapshot?.poll_seconds || "—"} secondes
          </KeyValue>
          <KeyValue label="Recherche CSV NAS">
            {s?.import_minutes || "—"} minutes
          </KeyValue>
          <KeyValue label="Fichiers importés">{value(s?.files, 0)}</KeyValue>
          <KeyValue label="Import en cours">
            {s?.import_active ? "Oui" : "Non"}
          </KeyValue>
          <button
            className="button"
            disabled={
              busy ||
              s?.import_active ||
              (!snapshot?.configured && !s?.nas_mounted)
            }
            onClick={runImport}
          >
            <RefreshCw
              size={16}
              className={s?.import_active ? "spinning" : ""}
            />
            {s?.import_active
              ? "Import en cours…"
              : busy
                ? "Démarrage…"
                : "Lancer un import"}
          </button>
          {started && (
            <p className="small-note" role="status">
              Import demandé. Les résultats apparaîtront dans le journal.
            </p>
          )}
        </Card>
      </div>
      <Card
        title="Un seul journal, toute l’histoire"
        eyebrow="Imports JSON et CSV"
        icon="activity"
        action={
          <button
            className="icon-button"
            aria-label="Actualiser le journal"
            onClick={resource.reload}
          >
            <RefreshCw size={17} />
          </button>
        }
      >
        {s?.runs?.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Source</th>
                  <th>Résultat</th>
                  <th>Lues</th>
                  <th>Ajoutées</th>
                  <th>Doublons</th>
                </tr>
              </thead>
              <tbody>
                {s.runs.map((run, i) => (
                  <tr key={run.id || i}>
                    <td>
                      <time dateTime={run.time_known === false ? undefined : run.at}>{run.time_known === false ? run.at_raw || run.at : dateLabel(run.at, {
                        day: "2-digit",
                        month: "short",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                        timeZone: s?.timezone || "Europe/Paris",
                      })}</time>
                      {run.time_known === false && <small className="table-error">Date source non reconnue</small>}
                    </td>
                    <td>{run.source}</td>
                    <td>
                      <Badge tone={run.error ? "red" : "green"}>
                        {run.error ? "À vérifier" : "Terminé"}
                      </Badge>
                      {run.error && (
                        <span className="table-error">{run.error}</span>
                      )}
                    </td>
                    <td>{run.read}</td>
                    <td className="quantity">{run.added}</td>
                    <td className="muted">{run.duplicates}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty title="Le journal est prêt">
            Les imports JSON et CSV partageront cet historique, sans liste
            parallèle.
          </Empty>
        )}
        {pagination && pagination.total > 0 && <nav className="import-pagination" aria-label="Pages du journal des imports">
          <span>{(pagination.page - 1) * pagination.page_size + 1}–{Math.min(pagination.page * pagination.page_size, pagination.total)} sur {value(pagination.total, 0)} imports</span>
          <div>
            <button className="button small" disabled={pagination.page <= 1 || resource.loading} onClick={() => setPage(pagination.page - 1)}>Précédents</button>
            <span>Page {pagination.page} / {pagination.pages}</span>
            <button className="button small" disabled={pagination.page >= pagination.pages || resource.loading} onClick={() => setPage(pagination.page + 1)}>Suivants</button>
          </div>
        </nav>}
        <p className="small-note">10 imports par page · Horaires {s?.timezone || "Europe/Paris"}, avec changement d’heure automatique. Les imports plus anciens restent accessibles par les pages du journal.</p>
      </Card>
      <div className="system-footer">
        <span>
          <Server size={15} /> Go + SQLite · Frontend embarqué · Sans Node.js
          sur le routeur
        </span>
        <span>OKOKO {snapshot?.version || "3.0.1"}</span>
      </div>
    </>
  );
}
