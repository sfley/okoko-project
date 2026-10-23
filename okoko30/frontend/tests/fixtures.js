// Synthetic data exclusively for browser tests. Never imported by application code.
export function fixtures() {
  const now = new Date();
  const iso = (d) => d.toISOString();
  const time = now.toISOString();
  const circuits = Object.fromEntries(
    ["hk1", "hk2", "ww1"].map((c, i) => [
      c,
      {
        name: ["Chambres", "Plancher", "ECS"][i],
        temperature: [20.8, 19.6, 52.4][i],
        setpoint: ["21.0 °C", "19.5 °C", "48.0 °C"][i],
        status: "Mode confort|Température souhaitée atteinte",
        pump: i < 2,
        flow: [34.6, 31.2, null][i],
        minimum: i === 2 ? 48 : null,
        maximum: i === 2 ? 60 : null,
      },
    ]),
  );
  const snapshot = {
    version: "3.0.1",
    instance_label: "BANC DE VALIDATION · DONNÉES SIMULÉES",
    configured: true,
    connected: true,
    last_seen: time,
    stale: false,
    poll_seconds: 60,
    outdoor: 12.8,
    boiler: {
      temperature: 64.2,
      modulation: 36,
      flame: 312,
      airflow: 42,
      pressure: 78,
      pump: 40,
      runtime: 18437,
      starts: 4218,
      average_runtime: 47,
      status: "Chauffage actif|Fonctionnement modulant",
    },
    circuits,
    pellets: { stock: 1470, capacity: 2200, today: 8.4, yesterday: 10.2 },
  };
  const history = Array.from({ length: 144 }, (_, i) => ({
    ts: iso(new Date(+now - (143 - i) * 600000)),
    outdoor: 12 + Math.sin(i / 30) * 3,
    hk1: 20.2 + Math.sin(i / 16) * 0.6,
    hk2: 19 + Math.sin(i / 20) * 0.5,
    ww1: 48 + Math.sin(i / 10) * 5,
    modulation: 32 + Math.sin(i / 13) * 18,
    stock: 1500 - i * 0.2,
  }));
  const forecast = Array.from({ length: 4 }, (_, i) => {
    const d = new Date(now);
    d.setDate(d.getDate() + i);
    return {
      date: `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`,
      temp: String([13, 14, 11, 15][i]),
      wind: String([8, 12, 7, 10][i]),
      condition: [
        "Peu nuageux",
        "Ciel dégagé",
        "Nuageux",
        "Partiellement nuageux",
      ][i],
    };
  });
  const transitions = ["hk1", "pe1", "hk2", "ww1"].map((c, i) => ({
    ts: iso(new Date(+now - i * 900000)),
    circuit: c,
    kind: i === 3 ? "pump" : "status0",
    before: "Mode réduit",
    after: i === 3 ? "0" : "Mode confort",
  }));
  const monthly = Array.from({ length: 12 }, (_, i) => ({
    year: now.getFullYear(),
    month: i + 1,
    kg: [320, 270, 221, 154, 72, 35, 23, 29, 86, 180, 260, 310][i],
  }));
  return {
    snapshot,
    history,
    weather: {
      current: { Lever: "07:52", Coucher: "18:27", "Durée du jour": "10h35" },
      forecast,
    },
    transitions,
    pellets: {
      monthly,
      refills: [
        {
          date: "2026-04-02T12:00:00",
          kg: 2000,
          supplier: "Fournisseur de test",
          kind: "Vrac",
          note: "Recharge simulée pour validation",
        },
      ],
    },
    system: {
      version: "3.0.1",
      sqlite: true,
      measurements: 184320,
      files: 128,
      nas_configured: true,
      nas_mounted: false,
      import_active: false,
      import_minutes: 60,
      runs: [
        {
          at: time,
          source: "JSON · log",
          added: 1420,
          error: "",
          read: 1440,
          duplicates: 20,
        },
        {
          at: time,
          source: "CSV · NAS",
          added: 0,
          error: "NAS indisponible (test)",
          read: 0,
          duplicates: 0,
        },
      ],
    },
  };
}
