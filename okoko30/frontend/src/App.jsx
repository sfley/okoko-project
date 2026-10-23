import React, { Suspense, lazy, useEffect, useState } from "react";
import {
  Activity,
  ArrowUpRight,
  Check,
  ChevronRight,
  CloudSun,
  Flame,
  LayoutDashboard,
  Leaf,
  Menu,
  RefreshCw,
  ServerCog,
  Settings2,
  Thermometer,
  WifiOff,
  X,
} from "lucide-react";
import { dateLabel, useResource, value } from "./api";
import { Badge, ErrorNotice } from "./components/UI";
import Cockpit from "./pages/Cockpit";
const Heating = lazy(() => import("./pages/Heating"));
const Analysis = lazy(() => import("./pages/Analysis"));
const Pellets = lazy(() => import("./pages/Pellets"));
const Weather = lazy(() => import("./pages/Weather"));
const System = lazy(() => import("./pages/System"));
const pages = [
  {
    path: "/",
    name: "Tableau de bord",
    icon: LayoutDashboard,
  },
  { path: "/chauffage", name: "Chauffage", icon: Flame },
  { path: "/analyse", name: "Analyse", icon: Activity },
  { path: "/pellets", name: "Pellets", icon: Leaf },
  { path: "/meteo", name: "Météo", icon: CloudSun },
  {
    path: "/system",
    name: "Système",
    icon: ServerCog,
  },
];
export default function App() {
  const [path, setPath] = useState(
    location.pathname === "/installation" ? "/chauffage" : location.pathname,
  );
  const [menu, setMenu] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const snapshot = useResource("snapshot", 15000);
  const weather = useResource("weather", 60000);
  useEffect(() => {
    const onPop = () => {
      setPath(location.pathname);
      setMenu(false);
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  useEffect(() => {
    document.title = `${pages.find((p) => p.path === path)?.name || "Supervision"} · OKOKO`;
    window.scrollTo(0, 0);
  }, [path]);
  useEffect(() => {
    const close = (e) => {
      const modal = document.querySelector('.modal[aria-modal="true"]');
      if (e.key === "Escape") {
        setMenu(false);
        if (modal) {
          const closeButton =
            modal.querySelector('button[aria-label^="Fermer"]') ||
            [...modal.querySelectorAll("button")].find(
              (b) => b.textContent === "Annuler",
            );
          closeButton?.click();
        }
      }
      if (e.key === "Tab" && modal) {
        const focusable = [
          ...modal.querySelectorAll(
            "button:not([disabled]),input:not([disabled]),select,a[href]",
          ),
        ];
        const first = focusable[0];
        const last = focusable.at(-1);
        if (
          e.shiftKey &&
          (document.activeElement === first ||
            !modal.contains(document.activeElement))
        ) {
          e.preventDefault();
          last?.focus();
        } else if (
          !e.shiftKey &&
          (document.activeElement === last ||
            !modal.contains(document.activeElement))
        ) {
          e.preventDefault();
          first?.focus();
        }
      }
    };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, []);
  const navigate = (event, next) => {
    if (
      event.button ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    history.pushState(null, "", next);
    setPath(next);
    setMenu(false);
  };
  async function refresh() {
    setRefreshing(true);
    await Promise.all([snapshot.reload(), weather.reload()]);
    setRefreshing(false);
  }
  const s = snapshot.data;
  const current = pages.find((p) => p.path === path) || pages[0];
  let body;
  switch (path) {
    case "/chauffage":
      body = <Heating snapshot={s} />;
      break;
    case "/analyse":
      body = <Analysis snapshot={s} />;
      break;
    case "/pellets":
      body = <Pellets snapshot={s} />;
      break;
    case "/meteo":
      body = <Weather weather={weather.data} snapshot={s} />;
      break;
    case "/system":
      body = <System snapshot={s} />;
      break;
    default:
      body = <Cockpit snapshot={s} weather={weather.data} />;
  }
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Aller au contenu
      </a>
      {menu && <div className="nav-scrim" onClick={() => setMenu(false)} />}
      <aside className={`sidebar ${menu ? "open" : ""}`}>
        <a
          className="brand"
          href="/"
          onClick={(e) => navigate(e, "/")}
          aria-label="OKOKO accueil"
        >
          <span className="brand-symbol">
            <Flame size={25} fill="currentColor" strokeWidth={1} />
          </span>
          <span>
            okoko<span className="brand-dot">.</span>
            <small>LA CHALEUR, SIMPLEMENT.</small>
          </span>
        </a>
        <button
          className="mobile-close icon-button"
          aria-label="Fermer la navigation"
          onClick={() => setMenu(false)}
        >
          <X size={22} />
        </button>
        <nav aria-label="Navigation principale">
          {pages.map((p) => (
            <React.Fragment key={p.path}>
              <a
                href={p.path}
                onClick={(e) => navigate(e, p.path)}
                className={path === p.path ? "active" : ""}
                aria-current={path === p.path ? "page" : undefined}
              >
                <p.icon size={19} strokeWidth={1.6} />
                <span>{p.name}</span>
                {path === p.path && <i />}
              </a>
            </React.Fragment>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="eco-card">
            <span>
              <Leaf size={18} />
              <strong>L’énergie du bon sens.</strong>
            </span>
            <p>
              Votre confort se pilote ici.
              <br />
              Vos données restent chez vous.
            </p>
            <div>
              <i /> Hébergé localement
            </div>
          </div>
          <div className="sidebar-version">
            <span>OKOKO</span>
            <span>Supervision ÖkoFEN</span>
          </div>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumbs">
            <button
              className="mobile-menu icon-button"
              aria-label="Ouvrir la navigation"
              onClick={() => setMenu(true)}
            >
              <Menu size={21} />
            </button>
            <span>OKOKO</span>
            <ChevronRight size={14} />
            <strong>{current.name}</strong>
          </div>
          <div className="topbar-right">
            <span className="outdoor">
              <CloudSun size={18} />
              {value(s?.outdoor)} °C<span>Extérieur</span>
            </span>
            <span className="topbar-divider" />
            <Badge tone={s?.connected ? "green" : "muted"} dot>
              {s?.connected ? "En ligne" : "En attente"}
            </Badge>
            <span className="user-avatar" aria-label="Installation locale">
              M
            </span>
          </div>
        </header>
        <main id="main-content" className="main-content" tabIndex={-1}>
          {s?.instance_label && (
            <div className="test-banner">{s.instance_label}</div>
          )}
          <div className="context-row">
            <span>
              {new Date().toLocaleDateString("fr-FR", {
                weekday: "long",
                day: "numeric",
                month: "long",
                year: "numeric",
              })}
            </span>
            <button
              onClick={refresh}
              className="refresh-status"
              disabled={refreshing}
              aria-label="Actualiser les données"
            >
              <RefreshCw size={13} className={refreshing ? "spinning" : ""} />
              {refreshing
                ? "Actualisation…"
                : s?.last_seen
                  ? `Dernière lecture ${dateLabel(s.last_seen, { hour: "2-digit", minute: "2-digit" })}`
                  : "Aucune lecture chaudière"}
            </button>
          </div>
          <ErrorNotice error={snapshot.error} retry={snapshot.reload} />
          {s?.stale && (
            <div className="error-notice">
              <WifiOff size={17} /> Données anciennes : les dernières valeurs
              reçues restent affichées.
            </div>
          )}
          <Suspense
            fallback={
              <div className="empty">
                <strong>Votre espace se prépare…</strong>
              </div>
            }
          >
            {body}
          </Suspense>
        </main>
      </div>
    </div>
  );
}
