import { test, expect } from "@playwright/test";
import { fixtures } from "./fixtures";
const routes = [
  ["/", "Tableau de bord"],
  ["/chauffage", "Chauffage"],
  ["/analyse", "Analyse"],
  ["/pellets", "Pellets"],
  ["/meteo", "Météo"],
  ["/system", "Système"],
];
async function mock(page, data = fixtures()) {
  await page.route("**/api/v3/**", (route) => {
    const key = new URL(route.request().url()).pathname.split("/").pop();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(data[key] ?? {}),
    });
  });
}
async function navigate(page, label, mobile = false) {
  if (mobile)
    await page.getByRole("button", { name: "Ouvrir la navigation" }).click();
  await page
    .getByRole("navigation")
    .getByRole("link", { name: label, exact: true })
    .click();
}
for (const viewport of [
  { name: "desktop", width: 1440, height: 1050 },
  { name: "tablet", width: 1024, height: 900 },
  { name: "mobile", width: 390, height: 844 },
]) {
  test(`${viewport.name}: six pages, no rendering error or overflow`, async ({
    page,
  }) => {
    await page.setViewportSize(viewport);
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    await mock(page);
    await page.goto("/");
    await expect(
      page.getByRole("heading", { name: /Votre maison/ }),
    ).toBeVisible();
    for (const [path, name] of routes) {
      if (path !== "/") await navigate(page, name, viewport.width <= 720);
      await expect(page.locator("main h1")).toBeVisible();
      await page.waitForTimeout(350);
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth + 1,
        ),
      ).toBe(true);
      if (path === "/") {
        for (const id of ["hk1", "hk2"]) {
          const widget = page.locator(`[data-widget="${id}"]`);
          const bounds = await widget.boundingBox();
          const graph = await widget.locator('.circuit-history').boundingBox();
          const footer = await widget.locator('.circuit-footer').boundingBox();
          expect(graph.height).toBeGreaterThan(25);
          expect(footer.y + footer.height).toBeLessThanOrEqual(bounds.y + bounds.height - 6);
        }
      }
      if (path === "/" && viewport.name === "tablet") {
        const pellets = await page.locator('[data-widget="pellets"]').boundingBox();
        const weather = await page.locator('[data-widget="weather"]').boundingBox();
        expect(Math.abs(pellets.y-weather.y)).toBeLessThan(2);
        expect(pellets.x).toBeLessThan(weather.x);
      }
      await page.screenshot({
        path: `../test-results/screenshots/${viewport.name}-${name.replaceAll(" ", "-")}.png`,
        fullPage: true,
      });
    }
    expect(errors).toEqual([]);
  });
}
test("cockpit: add, remove, resize, drag, reload persistence", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1540, height: 1100 });
  await mock(page);
  await page.goto("/");
  await page
    .getByRole("button", { name: "Personnaliser", exact: true })
    .click();
  const boiler = page.locator('[data-widget="boiler"]');
  const before = await boiler.boundingBox();
  await page
    .getByRole("button", { name: "Changer la taille Chaudière", exact: true })
    .click();
  await page.waitForTimeout(350);
  expect((await boiler.boundingBox()).height).not.toBe(before.height);
  await page
    .getByRole("button", { name: "Retirer Météo", exact: true })
    .click();
  await expect(page.locator('[data-widget="weather"]')).toHaveCount(0);
  await page
    .getByRole("button", { name: "Ajouter un module", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /Modulation/ })
    .click();
  await page.getByRole("button", { name: "C’est parfait" }).click();
  await expect(page.locator('[data-widget="modulation"]')).toHaveCount(1);
  const header = boiler.locator(".widget-drag");
  const box = await header.boundingBox();
  await page.mouse.move(box.x + 80, box.y + 20);
  await page.mouse.down();
  await page.mouse.move(box.x + 80, box.y + 360, { steps: 25 });
  await page.mouse.up();
  await page.waitForTimeout(500);
  const config = await page.evaluate(() =>
    localStorage.getItem("okoko30.cockpit.v1"),
  );
  expect(config).toContain("modulation");
  expect((await boiler.boundingBox()).y).not.toBe(before.y);
  const handle = boiler.locator(".react-resizable-handle");
  const hb = await handle.boundingBox();
  const oldSize = await boiler.boundingBox();
  await page.mouse.move(hb.x + 12, hb.y + 12);
  await page.mouse.down();
  await page.mouse.move(hb.x + 12, hb.y + 100, { steps: 15 });
  await page.mouse.up();
  await page.waitForTimeout(500);
  expect((await boiler.boundingBox()).height).toBeGreaterThan(oldSize.height);
  const finalConfig = await page.evaluate(() =>
    localStorage.getItem("okoko30.cockpit.v1"),
  );
  await page.getByRole("button", { name: "Terminer", exact: true }).click();
  await page.reload();
  await expect(page.locator('[data-widget="modulation"]')).toHaveCount(1);
  await expect(page.locator('[data-widget="weather"]')).toHaveCount(0);
  await page.waitForTimeout(250);
  expect(
    JSON.parse(
      await page.evaluate(() => localStorage.getItem("okoko30.cockpit.v1")),
    ).layouts.lg.map(({ i, x, y, w, h }) => ({ i, x, y, w, h })),
  ).toEqual(JSON.parse(finalConfig).layouts.lg.map(({ i, x, y, w, h }) => ({ i, x, y, w, h })));
});
test("missing data is not displayed as measured zero or invented forecast", async ({
  page,
}) => {
  const data = fixtures();
  data.snapshot = {
    configured: false,
    connected: false,
    boiler: {},
    circuits: {},
    pellets: {},
    last_seen: null,
  };
  data.weather = { current: {}, forecast: [] };
  data.history = [];
  data.transitions = [];
  await mock(page, data);
  await page.goto("/");
  await expect(
    page.getByText("Connexion chaudière à configurer"),
  ).toBeVisible();
  await expect(
    page.locator('[data-widget="boiler"] .boiler-reading'),
  ).toHaveText("—°C");
  await navigate(page, "Météo");
  await expect(
    page.locator(".forecast-grid").getByText("Non disponible", { exact: true }),
  ).toHaveCount(4);
  await expect(
    page.getByRole("heading", { name: "Au fil de la journée" }),
  ).toHaveCount(0);
});
test("heating never shows technical 8°C as comfort setpoint", async ({
  page,
}) => {
  const data = fixtures();
  data.snapshot.circuits.hk1.setpoint = "—";
  await mock(page, data);
  await page.goto("/");
  await navigate(page, "Chauffage");
  await expect(page.locator(".setpoint strong").first()).toHaveText("—");
  await expect(page.locator("main")).toContainText("19.5 °C");
});
test("chart legend and range controls respond", async ({ page }) => {
  await mock(page);
  await page.goto("/");
  await navigate(page, "Analyse");
  const legend = page.getByRole("button", { name: "Chambres", exact: true }).first();
  await legend.click();
  await expect(legend).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "7 jours", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "7 jours", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
});
test("refill form sends entered values without modifying existing records", async ({
  page,
}) => {
  await mock(page);
  let payload;
  await page.route("**/api/v3/refills", async (route) => {
    payload = route.request().postDataJSON();
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: '{"saved":true}',
    });
  });
  await page.goto("/");
  await navigate(page, "Pellets");
  await page
    .getByRole("button", { name: "Ajouter une recharge", exact: true })
    .click();
  await page.getByLabel("Date de livraison").fill("2026-10-21");
  await page.getByLabel("Quantité en kg").fill("330");
  await page.getByRole("button", { name: "Enregistrer la recharge" }).click();
  await expect(page.getByRole("status")).toContainText("Recharge enregistrée");
  expect(payload.kg).toBe(330);
  expect(payload.date).toBe("2026-10-21");
});
test("real backend serves SPA and unavailable NAS without invented telemetry", async ({
  page,
  request,
}) => {
  test.skip(
    process.env.OKOKO_REAL_BACKEND !== "1",
    "Requires actual Go server",
  );
  const health = await request.get("/health");
  expect(health.status()).toBe(200);
  const response = await request.get("/api/v3/snapshot");
  const body = await response.json();
  expect(body.boiler.temperature).toBeNull();
  for (const [path] of routes) {
    await page.goto(path);
    await expect(page.locator("main h1")).toBeVisible();
  }
  await expect(page.locator("main")).toContainText("NAS non configuré");
});

test('unified navigation and named circuits with shared mini histories', async ({ page }) => {
 const historyCalls = [];
 page.on('request', r => { if (r.url().includes('/api/v3/history')) historyCalls.push(r.url()); });
 await mock(page);
 await page.goto('/');
 await expect(page.locator('aside nav a')).toHaveCount(6);
 await expect(page.locator('.home-selector, .nav-label')).toHaveCount(0);
 for (const [id, name] of [['hk1','Chambres'],['hk2','Plancher']]) {
  const widget = page.locator(`[data-widget="${id}"]`);
  await expect(widget.locator('h2')).toHaveText(name);
  await expect(widget.getByRole('group', { name: `Historique 24 heures · ${name}` })).toBeVisible();
  await expect(widget.locator('.recharts-line-curve')).toHaveCount(1);
  const setpoint = await widget.locator('.setpoint').boundingBox();
  const graph = await widget.locator('.circuit-history').boundingBox();
  const pump = await widget.locator('.circuit-footer').boundingBox();
  expect(graph.y).toBeGreaterThanOrEqual(setpoint.y + setpoint.height);
  expect(graph.y + graph.height).toBeLessThanOrEqual(pump.y + 1);
 }
 expect(historyCalls).toHaveLength(1);
 await expect(page.getByRole('button', {name:'Extérieur',exact:true})).toHaveAttribute('aria-pressed','true');
 await navigate(page,'Chauffage');
 await expect(page.locator('.circuit-sparkline .recharts-line-curve')).toHaveCount(2);
 expect(historyCalls).toHaveLength(2);
 await navigate(page,'Analyse');
 await expect(page.getByRole('button', {name:'Extérieur',exact:true})).toBeVisible();
});

test('imports: ten rows per page and all older records accessible', async ({ page }) => {
 const data = fixtures();
 await mock(page,data);
 await page.route('**/api/v3/system?*', route => {
  const pageNo = Number(new URL(route.request().url()).searchParams.get('page') || 1);
  const all = Array.from({length:25},(_,i)=>({id:25-i,at:'2026-10-23T07:50:00Z',time_known:true,source:`CSV ${25-i}`,read:1,added:1,duplicates:0,error:''}));
  return route.fulfill({json:{...data.system,timezone:'Europe/Paris',runs:all.slice((pageNo-1)*10,pageNo*10),pagination:{page:pageNo,page_size:10,pages:3,total:25}}});
 });
 await page.goto('/system');
 await expect(page.locator('tbody tr')).toHaveCount(10);
 await expect(page.locator('tbody tr').first()).toContainText('CSV 25');
 await expect(page.locator('tbody tr').first()).toContainText('09:50:00');
 await page.screenshot({path:'../test-results/screenshots/desktop-Imports-10-lignes.png',fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await page.waitForTimeout(350);
 await page.screenshot({path:'../test-results/screenshots/mobile-Imports-10-lignes.png',fullPage:true});
 await page.getByRole('button',{name:'Suivants',exact:true}).click();
 await expect(page.locator('tbody tr').first()).toContainText('CSV 15');
 await expect(page.locator('tbody tr')).toHaveCount(10);
 await page.getByRole('button',{name:'Suivants',exact:true}).click();
 await expect(page.locator('tbody tr')).toHaveCount(5);
 await expect(page.locator('tbody tr').last()).toContainText('CSV 1');
 await expect(page.getByRole('button',{name:'Suivants',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Précédents',exact:true}).click();
 await expect(page.locator('tbody tr')).toHaveCount(10);
});

test('import clock follows Paris in summer and winter regardless of browser timezone', async ({ browser }) => {
 const context = await browser.newContext({timezoneId:'Asia/Tokyo'});
 const page = await context.newPage();
 const data = fixtures();
 data.system.timezone = 'Europe/Paris';
 data.system.runs = [
  {id:2,at:'2026-10-23T07:50:00Z',time_known:true,source:'UTC été',added:1,read:1,duplicates:0},
  {id:1,at:'2026-12-23T07:50:00Z',time_known:true,source:'UTC hiver',added:1,read:1,duplicates:0},
 ];
 await mock(page,data);
 await page.goto('/system');
 await expect(page.locator('tbody tr').first()).toContainText('09:50:00');
 await expect(page.locator('tbody tr').last()).toContainText('08:50:00');
 await context.close();
});

test('real Go parser: supplied raw JSON, French forecasts and named circuits', async ({ page, request }) => {
 test.skip(process.env.OKOKO_RAW_FIXTURE !== '1', 'Requires isolated Go fixture server');
 const base = 'http://127.0.0.1:8889';
 const response = await request.get(`${base}/api/v3/weather`);
 const weather = await response.json();
 expect(weather.forecast).toHaveLength(25);
 expect(weather.forecast[0].timestamp).toBe('2026-10-23T09:44:00+02:00');
 expect(weather.forecast[24].day).toBe('2026-10-26');
 await page.goto(`${base}/chauffage`);
 await expect(page.getByRole('heading',{name:'Chambres',exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'Plancher',exact:true})).toBeVisible();
 await expect(page.locator('.setpoint strong').first()).toHaveText('19.6 °C');
 await navigate(page,'Météo');
 await expect(page.locator('.weather-hero-reading')).toContainText('14');
 await expect(page.locator('.forecast-grid .forecast-temp')).toHaveCount(4);
 await expect(page.locator('.weather-hero-details')).toContainText('08:31');
 await expect(page.locator('.weather-hero-details')).toContainText('57 km/h');
 await expect(page.locator('main')).not.toContainText('km/h km/h');
 const monday = page.locator('.periods-grid section').filter({has:page.getByRole('heading',{name:/lundi/})});
 await expect(monday.locator('.weather-period')).toHaveCount(1);
 await expect(monday).toContainText('Matin · 07:00');
 await expect(monday).not.toContainText('Midi');
 await expect(monday).not.toContainText('Soir');
 await page.screenshot({path:'../test-results/screenshots/raw-json-Météo.png',fullPage:true});
});
