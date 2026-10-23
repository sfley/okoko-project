// Only explicit dates/times from the boiler are interpreted. No inferred hourly weather.
export function parseForecastDate(raw) {
  if (!raw) return null;
  const value = String(raw).trim();
  let match = value.match(
    /^(\d{1,2})[./](\d{1,2})[./](\d{4})(?:[ T]+(\d{1,2}):(\d{2}))?/,
  );
  if (match) {
    const [, d, m, y, h, minute] = match;
    const dt = new Date(+y, +m - 1, +d, +(h || 12), +(minute || 0));
    return Number.isNaN(+dt) ||
      dt.getFullYear() !== +y ||
      dt.getMonth() !== +m - 1 ||
      dt.getDate() !== +d ||
      (h !== undefined && (dt.getHours() !== +h || dt.getMinutes() !== +minute))
      ? null
      : { date: dt, hourly: h !== undefined };
  }
  match = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T]+(\d{1,2}):(\d{2}))?/);
  if (match) {
    const [, y, m, d, h, minute] = match;
    const dt = new Date(+y, +m - 1, +d, +(h || 12), +(minute || 0));
    return Number.isNaN(+dt) ||
      dt.getFullYear() !== +y ||
      dt.getMonth() !== +m - 1 ||
      dt.getDate() !== +d ||
      (h !== undefined && (dt.getHours() !== +h || dt.getMinutes() !== +minute))
      ? null
      : { date: dt, hourly: h !== undefined };
  }
  return null;
}

export function forecastNumber(raw) {
  if (raw === null || raw === undefined || String(raw).trim() === "")
    return null;
  const n = Number(String(raw).replace(",", "."));
  return Number.isFinite(n) ? n : null;
}

export function windLabel(raw) {
  if (raw === null || raw === undefined || String(raw).trim() === "") return "—";
  const text = String(raw).trim();
  return forecastNumber(text) !== null ? `${text} km/h` : text;
}
