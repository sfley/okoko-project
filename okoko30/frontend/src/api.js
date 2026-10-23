import { useCallback, useEffect, useRef, useState } from "react";
export async function request(path, options = {}) {
  const response = await fetch(`/api/v3/${path}`, {
    ...options,
    signal: options.signal ?? AbortSignal.timeout(10000),
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...options.headers,
    },
  });
  if (!response.ok)
    throw new Error(
      (await response.text()).trim() || `Erreur ${response.status}`,
    );
  return response.json();
}
export function useResource(path, interval = 0) {
  const [state, setState] = useState({ data: null, error: "", loading: true });
  const active = useRef(null);
  const reload = useCallback(async () => {
    if (!path || active.current) return;
    const controller = new AbortController();
    active.current = controller;
    const timeout = setTimeout(() => controller.abort(), 10000);
    try {
      const data = await request(path, { signal: controller.signal });
      if (active.current === controller)
        setState({ data, error: "", loading: false });
    } catch (error) {
      if (active.current === controller)
        setState((old) => ({
          ...old,
          error:
            error.name === "AbortError"
              ? "Le serveur met trop de temps à répondre."
              : error.message,
          loading: false,
        }));
    } finally {
      clearTimeout(timeout);
      if (active.current === controller) active.current = null;
    }
  }, [path]);
  useEffect(() => {
    setState({ data: null, error: "", loading: true });
    reload();
    const refresh = () => {
      if (!document.hidden) reload();
    };
    const timer = interval ? setInterval(refresh, interval) : null;
    document.addEventListener("visibilitychange", refresh);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
      active.current?.abort();
      active.current = null;
    };
  }, [reload, interval]);
  return { ...state, reload };
}
export const value = (n, digits = 1) =>
  typeof n === "number" && Number.isFinite(n)
    ? new Intl.NumberFormat("fr-FR", { maximumFractionDigits: digits }).format(
        n,
      )
    : "—";
export const dateLabel = (raw, options = {}) => {
  if (!raw) return "Aucune lecture";
  const date = new Date(raw.replace(" ", "T"));
  return Number.isNaN(+date) ? raw : date.toLocaleString("fr-FR", { timeZone: "Europe/Paris", ...options });
};
