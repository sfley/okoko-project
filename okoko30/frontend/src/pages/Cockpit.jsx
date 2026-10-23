import React, { useEffect, useState } from "react";
import { Responsive, WidthProvider } from "react-grid-layout";
import {
  ArrowDown,
  ArrowUp,
  Check,
  ChevronDown,
  Grip,
  Maximize2,
  Minimize2,
  Plus,
  RotateCcw,
  Settings2,
  X,
} from "lucide-react";
import { useResource } from "../api";
import { Badge, Icon, PageHeading, SavedIndicator } from "../components/UI";
import { HistoryChart } from "../components/Charts";
import {
  ActivityWidget,
  BoilerWidget,
  CircuitWidget,
  ModulationWidget,
  PelletsWidget,
  SystemWidget,
  WeatherWidget,
} from "../components/Widgets";
const Grid = WidthProvider(Responsive);
export const LIBRARY = [
  {
    id: "boiler",
    name: "Chaudière",
    sub: "Température, état et puissance",
    w: 6,
    h: 4,
  },
  {
    id: "pellets",
    name: "Réserve de pellets",
    sub: "Un œil sur votre silo",
    w: 3,
    h: 4,
  },
  { id: "weather", name: "Météo", sub: "Les prévisions ÖkoFEN", w: 3, h: 4 },
  { id: "hk1", name: "Circuit HK1", sub: "Confort et température", w: 4, h: 3 },
  { id: "hk2", name: "Circuit HK2", sub: "Confort et température", w: 4, h: 3 },
  {
    id: "ecs",
    name: "Eau chaude",
    sub: "Eau chaude sanitaire · ECS",
    w: 4,
    h: 3,
  },
  {
    id: "temperatures",
    name: "Températures",
    sub: "L’équilibre de votre maison",
    w: 8,
    h: 4,
  },
  {
    id: "activity",
    name: "Activité",
    sub: "Les derniers changements",
    w: 4,
    h: 4,
  },
  {
    id: "modulation",
    name: "Modulation",
    sub: "L’allure de votre chaudière",
    w: 4,
    h: 4,
  },
  {
    id: "system",
    name: "Système",
    sub: "La santé de votre installation",
    w: 4,
    h: 4,
  },
];
const DEFAULT_IDS = [
  "boiler",
  "pellets",
  "weather",
  "hk1",
  "hk2",
  "ecs",
  "temperatures",
  "activity",
];
const STORAGE = "okoko30.cockpit.v1";
function defaultLayout() {
  return [
    { i: "boiler", x: 0, y: 0, w: 6, h: 4 },
    { i: "pellets", x: 6, y: 0, w: 3, h: 4 },
    { i: "weather", x: 9, y: 0, w: 3, h: 4 },
    { i: "hk1", x: 0, y: 4, w: 4, h: 3 },
    { i: "hk2", x: 4, y: 4, w: 4, h: 3 },
    { i: "ecs", x: 8, y: 4, w: 4, h: 3 },
    { i: "temperatures", x: 0, y: 7, w: 8, h: 4 },
    { i: "activity", x: 8, y: 7, w: 4, h: 4 },
  ].map((x) => ({ ...x, minW: 2, minH: 3, maxH: 7 }));
}
function defaultLayouts() {
  const lg = defaultLayout();
  const make = (columns) => {
    if (columns === 1) {
      let y = 0;
      return DEFAULT_IDS.map((id) => {
        const item = LIBRARY.find((x) => x.id === id);
        const row = {
          i: id,
          x: 0,
          y,
          w: 1,
          h: item.h,
          minW: 1,
          minH: 3,
          maxH: 7,
        };
        y += item.h;
        return row;
      });
    }
    const half = columns / 2;
    return [
      { i: "boiler", x: 0, y: 0, w: columns, h: 4 },
      { i: "pellets", x: 0, y: 4, w: half, h: 4 },
      { i: "weather", x: half, y: 4, w: half, h: 4 },
      { i: "hk1", x: 0, y: 8, w: half, h: 3 },
      { i: "hk2", x: half, y: 8, w: half, h: 3 },
      { i: "ecs", x: 0, y: 11, w: columns, h: 3 },
      { i: "temperatures", x: 0, y: 14, w: columns, h: 4 },
      { i: "activity", x: 0, y: 18, w: columns, h: 3 },
    ].map((row) => ({ ...row, minW: columns === 2 ? 1 : 2, minH: 3, maxH: 7 }));
  };
  return { lg, md: make(6), sm: make(2), xs: make(1) };
}
function initial() {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE));
    if (
      saved?.version === 1 &&
      Array.isArray(saved.ids) &&
      saved.ids.every((id) => LIBRARY.some((x) => x.id === id)) &&
      new Set(saved.ids).size === saved.ids.length &&
      saved.layouts &&
      typeof saved.layouts === "object"
    ) {
      const layouts = {};
      for (const [key, rows] of Object.entries(saved.layouts))
        if (["lg", "md", "sm", "xs"].includes(key) && Array.isArray(rows))
          layouts[key] = rows
            .filter(
              (row) =>
                saved.ids.includes(row.i) &&
                ["x", "y", "w", "h"].every(
                  (k) => Number.isFinite(row[k]) && row[k] >= 0,
                ) &&
                row.w >= 1 &&
                row.h >= 3,
            )
            .map((row) => ({
              i: row.i,
              x: row.x,
              y: row.y,
              w: row.w,
              h: row.h,
              minW: key === "xs" || key === "sm" ? 1 : 2,
              minH: 3,
              maxH: 7,
            }));
      if (layouts.lg?.length === saved.ids.length)
        return { ids: saved.ids, layouts };
    }
  } catch {
    /* An invalid or blocked store must never prevent rendering. */
  }
  return { ids: DEFAULT_IDS, layouts: defaultLayouts() };
}
export default function Cockpit({ snapshot, weather }) {
  const [config, setConfig] = useState(initial);
  const [editing, setEditing] = useState(false);
  const [library, setLibrary] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [reset, setReset] = useState(false);
  const history = useResource("history?days=1", 60000);
  const libraryItems = LIBRARY.map((item) => ({ ...item, name: snapshot?.circuits?.[item.id === "ecs" ? "ww1" : item.id]?.name || item.name }));
  const transitions = useResource("transitions", 60000);
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE, JSON.stringify({ version: 1, ...config }));
      setSaveError("");
    } catch {
      setSaveError("Sauvegarde locale indisponible");
    }
  }, [config]);
  function toggle(id) {
    setConfig((old) => {
      if (old.ids.includes(id))
        return {
          ids: old.ids.filter((x) => x !== id),
          layouts: Object.fromEntries(
            Object.entries(old.layouts).map(([k, rows]) => [
              k,
              rows.filter((x) => x.i !== id),
            ]),
          ),
        };
      const item = LIBRARY.find((x) => x.id === id);
      return {
        ids: [...old.ids, id],
        layouts: Object.fromEntries(
          Object.entries(old.layouts).map(([bp, rows]) => {
            const columns = { lg: 12, md: 6, sm: 2, xs: 1 }[bp];
            const width =
              bp === "lg"
                ? item.w
                : bp === "md"
                  ? item.w >= 6
                    ? 6
                    : 3
                  : bp === "sm"
                    ? item.w >= 6
                      ? 2
                      : 1
                    : 1;
            return [
              bp,
              [
                ...rows,
                {
                  i: id,
                  x: 0,
                  y: rows.reduce((max, row) => Math.max(max, row.y + row.h), 0),
                  w: width,
                  h: item.h,
                  minW: columns <= 2 ? 1 : 2,
                  minH: 3,
                  maxH: 7,
                },
              ],
            ];
          }),
        ),
      };
    });
  }
  function size(id) {
    setConfig((old) => ({
      ...old,
      layouts: Object.fromEntries(
        Object.entries(old.layouts).map(([key, rows]) => [
          key,
          rows.map((row) =>
            row.i === id
              ? {
                  ...row,
                  w:
                    row.w <= 4
                      ? Math.min(
                          key === "lg" ? 12 : key === "md" ? 6 : 2,
                          key === "lg" ? 6 : key === "md" ? 6 : 2,
                        )
                      : 4,
                  h: row.h <= 3 ? 4 : 3,
                }
              : row,
          ),
        ]),
      ),
    }));
  }
  function move(id, delta) {
    setConfig((old) => {
      const ids = [...old.ids];
      const i = ids.indexOf(id);
      const j = i + delta;
      if (j < 0 || j >= ids.length) return old;
      const layouts = {};
      for (const [bp, rows] of Object.entries(old.layouts)) {
        const a = rows.find((x) => x.i === id),
          b = rows.find((x) => x.i === ids[j]);
        layouts[bp] = rows.map((row) =>
          row.i === id && b
            ? { ...row, x: b.x, y: b.y }
            : row.i === ids[j] && a
              ? { ...row, x: a.x, y: a.y }
              : row,
        );
      }
      [ids[i], ids[j]] = [ids[j], ids[i]];
      return { ids, layouts };
    });
  }
  const renderWidget = (id) => {
    if (id === "boiler") return <BoilerWidget snapshot={snapshot} />;
    if (["hk1", "hk2", "ecs"].includes(id))
      return (
        <CircuitWidget
          snapshot={snapshot}
          circuit={id === "ecs" ? "ww1" : id}
          history={history.data || []}
        />
      );
    if (id === "pellets") return <PelletsWidget snapshot={snapshot} />;
    if (id === "weather")
      return <WeatherWidget snapshot={snapshot} weather={weather} />;
    if (id === "temperatures")
      return <HistoryChart compact data={history.data || []} snapshot={snapshot} />;
    if (id === "activity")
      return <ActivityWidget transitions={transitions.data || []} snapshot={snapshot} />;
    if (id === "modulation")
      return (
        <ModulationWidget snapshot={snapshot} history={history.data || []} />
      );
    return <SystemWidget snapshot={snapshot} />;
  };
  return (
    <>
      <PageHeading
        eyebrow="L’essentiel, en un regard"
        title={
          <>
            Votre maison, <span>en équilibre.</span>
          </>
        }
        description="Un peu de chaleur. Beaucoup de sérénité."
      >
        <button
          className={editing ? "button primary" : "button"}
          onClick={() => {
            setEditing(!editing);
            setLibrary(false);
          }}
          aria-pressed={editing}
        >
          {editing ? <Check size={16} /> : <Settings2 size={16} />}{" "}
          {editing ? "Terminer" : "Personnaliser"}
        </button>
      </PageHeading>
      <div className="dashboard-strip">
        <div>
          <span
            className={`live-dot ${snapshot?.connected ? "connected" : ""}`}
          />
          <span>
            {snapshot?.connected
              ? "Votre installation est connectée"
              : snapshot?.configured
                ? "Connexion chaudière en attente"
                : "Connexion chaudière à configurer"}
          </span>
          <span className="strip-divider" />
          <span className="muted">ÖkoFEN Pellematic PE CONDENS</span>
        </div>
        <SavedIndicator error={saveError} />
      </div>
      {editing && (
        <div className="edit-toolbar">
          <span>
            <Grip size={16} /> Déplacez les modules par leur en-tête et tirez
            leur coin pour les redimensionner.
          </span>
          <button className="button small" onClick={() => setReset(true)}>
            <RotateCcw size={14} /> Réinitialiser
          </button>
          <button
            className="button primary small"
            onClick={() => setLibrary(true)}
          >
            <Plus size={15} /> Ajouter un module
          </button>
        </div>
      )}
      {!config.ids.length && (
        <div className="empty-cockpit">
          <Icon name="temperatures" size={35} />
          <h2>Votre cockpit, votre rythme.</h2>
          <p>Choisissez les informations qui comptent pour vous.</p>
          <button
            className="button primary"
            onClick={() => {
              setEditing(true);
              setLibrary(true);
            }}
          >
            <Plus size={16} /> Ajouter un module
          </button>
        </div>
      )}
      <Grid
        className={`cockpit-grid ${editing ? "editing" : ""}`}
        layouts={config.layouts}
        breakpoints={{ lg: 1050, md: 700, sm: 440, xs: 0 }}
        cols={{ lg: 12, md: 6, sm: 2, xs: 1 }}
        rowHeight={64}
        margin={[18, 18]}
        containerPadding={[0, 0]}
        isDraggable={editing}
        isResizable={editing}
        draggableHandle=".widget-drag"
        draggableCancel="button,a,input"
        resizeHandles={["se"]}
        onLayoutChange={(current, all) => {
          setConfig((old) =>
            JSON.stringify(old.layouts) === JSON.stringify(all)
              ? old
              : { ...old, layouts: all },
          );
        }}
        useCSSTransforms
      >
        {config.ids.map((id) => {
          const item = libraryItems.find((x) => x.id === id);
          return (
            <section
              key={id}
              className={`widget widget-${id}`}
              data-widget={id}
              aria-label={item.name}
            >
              <div className="widget-heading widget-drag">
                <h2>
                  <span className={`widget-icon ${id}`}>
                    <Icon name={id} />
                  </span>
                  {item.name}
                </h2>
                {!editing ? (
                  <span className="widget-kicker">
                    {id === "boiler"
                      ? "TEMPS RÉEL"
                      : id === "temperatures"
                        ? "24 HEURES"
                        : ""}
                  </span>
                ) : (
                  <div className="widget-controls">
                    <button
                      title={`Déplacer ${item.name} avant`}
                      aria-label={`Déplacer ${item.name} avant`}
                      onClick={() => move(id, -1)}
                    >
                      <ArrowUp size={13} />
                    </button>
                    <button
                      title={`Déplacer ${item.name} après`}
                      aria-label={`Déplacer ${item.name} après`}
                      onClick={() => move(id, 1)}
                    >
                      <ArrowDown size={13} />
                    </button>
                    <button
                      aria-label={`Changer la taille ${item.name}`}
                      title="Compact / détaillé"
                      onClick={() => size(id)}
                    >
                      <Maximize2 size={13} />
                    </button>
                    <button
                      aria-label={`Retirer ${item.name}`}
                      onClick={() => toggle(id)}
                    >
                      <X size={14} />
                    </button>
                  </div>
                )}
              </div>
              <div className="widget-content">{renderWidget(id)}</div>
            </section>
          );
        })}
      </Grid>
      <div className="dashboard-note">
        <span>Conçu pour le confort. Pensé pour durer.</span>
        <span>Collecte espacée · Données conservées localement</span>
      </div>
      {library && (
        <div
          className="modal-backdrop"
          onClick={(e) => e.target === e.currentTarget && setLibrary(false)}
        >
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="library-title"
            className="modal library-modal"
          >
            <div className="modal-heading">
              <div>
                <span className="eyebrow">À votre mesure</span>
                <h2 id="library-title">Bibliothèque de modules</h2>
                <p>Composez le cockpit qui vous ressemble.</p>
              </div>
              <button
                className="icon-button"
                aria-label="Fermer la bibliothèque"
                onClick={() => setLibrary(false)}
                autoFocus
              >
                <X size={21} />
              </button>
            </div>
            <div className="library-grid">
              {libraryItems.map((item) => (
                <button
                  key={item.id}
                  className={`library-item ${config.ids.includes(item.id) ? "selected" : ""}`}
                  aria-pressed={config.ids.includes(item.id)}
                  onClick={() => toggle(item.id)}
                >
                  <span className={`widget-icon ${item.id}`}>
                    <Icon name={item.id} size={22} />
                  </span>
                  <strong>{item.name}</strong>
                  <span>{item.sub}</span>
                  <i>
                    {config.ids.includes(item.id) ? (
                      <Check size={17} />
                    ) : (
                      <Plus size={17} />
                    )}
                  </i>
                </button>
              ))}
            </div>
            <div className="modal-footer">
              <span>Enregistré automatiquement dans ce navigateur.</span>
              <button
                className="button primary"
                onClick={() => setLibrary(false)}
              >
                C’est parfait <Check size={16} />
              </button>
            </div>
          </section>
        </div>
      )}
      {reset && (
        <div className="modal-backdrop">
          <section
            role="dialog"
            aria-modal="true"
            className="modal confirmation"
          >
            <h2>Retrouver le cockpit d’origine ?</h2>
            <p>
              Votre disposition personnalisée sera remplacée. Les données de
              votre installation restent conservées.
            </p>
            <div className="modal-footer">
              <button className="button" onClick={() => setReset(false)}>
                Annuler
              </button>
              <button
                className="button primary"
                onClick={() => {
                  setConfig({
                    ids: DEFAULT_IDS,
                    layouts: defaultLayouts(),
                  });
                  setReset(false);
                }}
              >
                Réinitialiser le cockpit
              </button>
            </div>
          </section>
        </div>
      )}
    </>
  );
}
