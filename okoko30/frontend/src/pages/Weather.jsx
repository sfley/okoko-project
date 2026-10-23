import React from "react";
import { CalendarDays, Cloud, Sunrise, Sunset, Wind } from "lucide-react";
import {
  Badge,
  Card,
  Empty,
  KeyValue,
  PageHeading,
  WeatherIcon,
} from "../components/UI";
import { value } from "../api";
import { parseForecastDate, forecastNumber, windLabel } from "../weather-data";
export default function Weather({ weather, snapshot }) {
  const today = parseForecastDate(weather?.current_day)?.date || new Date();
  today.setHours(0, 0, 0, 0);
  const entries = (weather?.forecast || []).map((f) => ({
    ...f,
    parsed: parseForecastDate(f.date),
  }));
  const dateKey = (d) => `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
  const days = Array.from({ length: 4 }, (_, i) => {
    const date = new Date(today);
    date.setDate(date.getDate() + i);
    return {
      date,
      entries: entries.filter(
        (e) => e.parsed && dateKey(e.parsed.date) === dateKey(date),
      ),
    };
  });
  const current = days[0].entries[0];
  const temperature =
    current?.temp !== undefined
      ? forecastNumber(current.temp)
      : snapshot?.outdoor;
  const unscheduled = entries.filter((e) => !e.parsed);
  return (
    <>
      <PageHeading
        eyebrow="La maison et les saisons"
        title={
          <>
            Un œil dehors. <span>Le confort dedans.</span>
          </>
        }
        description="La météo transmise par votre chaudière, sans service externe ni prévision inventée."
      >
        <Badge>
          <Cloud size={13} /> Source ÖkoFEN
        </Badge>
      </PageHeading>
      <div className="weather-hero">
        <div className="weather-hero-main">
          <span className="eyebrow">
            Aujourd’hui ·{" "}
            {today.toLocaleDateString("fr-FR", {
              day: "numeric",
              month: "long",
            })}
          </span>
          <div className="weather-hero-reading">
            <WeatherIcon condition={current?.condition} size={85} />
            <span>
              {value(temperature)}
              <small>°C</small>
            </span>
          </div>
          <h2>{current?.condition || "Prévisions en attente"}</h2>
          <p>
            {current
              ? "Les données de votre installation, simplement."
              : "Aucune prévision datée disponible pour aujourd’hui."}
          </p>
        </div>
        <div className="weather-hero-details">
          <div>
            <Sunrise size={23} />
            <span>Lever du soleil</span>
            <strong>
              {current && current.is_current !== false && weather?.current?.Lever && weather.current.Lever !== "—"
                ? weather.current.Lever
                : "—"}
            </strong>
          </div>
          <div>
            <Sunset size={23} />
            <span>Coucher du soleil</span>
            <strong>
              {current && current.is_current !== false && weather?.current?.Coucher && weather.current.Coucher !== "—"
                ? weather.current.Coucher
                : "—"}
            </strong>
          </div>
          <div>
            <Wind size={23} />
            <span>Vent annoncé</span>
            <strong>{windLabel(current?.wind)}</strong>
          </div>
        </div>
        <div className="weather-hero-decoration" aria-hidden="true">
          <div />
          <div />
          <div />
        </div>
      </div>
      <div className="section-title">
        <h2>Les jours qui viennent</h2>
        <span>Aujourd’hui et les trois prochains jours</span>
      </div>
      <div className="forecast-grid">
        {days.map((day, i) => {
          const forecast = day.entries[0];
          const temps = day.entries
            .map((x) => forecastNumber(x.temp))
            .filter((n) => n !== null);
          return (
            <Card
              key={i}
              title={
                i === 0
                  ? "Aujourd’hui"
                  : day.date.toLocaleDateString("fr-FR", { weekday: "long" })
              }
              eyebrow={day.date.toLocaleDateString("fr-FR", {
                day: "numeric",
                month: "short",
              })}
              className={i === 0 ? "forecast-today" : ""}
            >
              {forecast ? (
                <>
                  <WeatherIcon condition={forecast.condition} size={46} />
                  <div className="forecast-temp">
                    {value(temps.length ? temps[0] : null)}
                    <small>°C</small>
                  </div>
                  <strong>{forecast.condition}</strong>
                  {forecast.parsed?.hourly && <span className="small-note">À {forecast.parsed.date.toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit" })}</span>}
                  <span className="forecast-wind">
                    <Wind size={13} />{" "}
                    {forecast.wind
                      ? windLabel(forecast.wind)
                      : "Vent non renseigné"}
                  </span>
                  {temps.length > 1 && (
                    <span className="small-note">
                      Mesures prévues : {value(Math.min(...temps))} à{" "}
                      {value(Math.max(...temps))} °C
                    </span>
                  )}
                </>
              ) : (
                <Empty compact icon={Cloud} title="Non disponible">
                  La chaudière n’a pas transmis de prévision pour ce jour.
                </Empty>
              )}
            </Card>
          );
        })}
      </div>
      {days.some((d) => d.entries.some((e) => e.parsed.hourly)) && (
        <Card
          title="Au fil de la journée"
          eyebrow="Détails horaires réellement disponibles"
          icon="weather"
        >
          <div className="periods-grid">
            {days.map((day) => {
              const slots = [
                ["Matin", 5, 11],
                ["Midi", 11, 15],
                ["Soir", 15, 23],
              ]
                .map(([label, start, end]) => ({
                  label,
                  data: day.entries.find(
                    (e) =>
                      e.parsed.hourly &&
                      e.parsed.date.getHours() >= start &&
                      e.parsed.date.getHours() < end,
                  ),
                }))
                .filter((x) => x.data);
              return slots.length ? (
                <section key={String(day.date)}>
                  <h3>
                    {day.date.toLocaleDateString("fr-FR", {
                      weekday: "long",
                      day: "numeric",
                    })}
                  </h3>
                  {slots.map(({ label, data }) => (
                    <div className="weather-period" key={label}>
                      <span>
                        {label} ·{" "}
                        {data.parsed.date.toLocaleTimeString("fr-FR", {
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                      <WeatherIcon condition={data.condition} size={26} />
                      <strong>{data.temp} °C</strong>
                      <small>{data.condition}</small>
                    </div>
                  ))}
                </section>
              ) : null;
            })}
          </div>
        </Card>
      )}
      {unscheduled.length > 0 && (
        <Card title="Prévisions au format non daté" icon="weather">
          <p className="small-note">
            Ces données sont affichées telles que transmises ; elles ne sont pas
            affectées arbitrairement à un jour.
          </p>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Date transmise</th>
                  <th>Température</th>
                  <th>Conditions</th>
                </tr>
              </thead>
              <tbody>
                {unscheduled.map((entry, i) => (
                  <tr key={i}>
                    <td>{entry.date}</td>
                    <td>{entry.temp} °C</td>
                    <td>{entry.condition}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
      <div className="info-banner">
        <CalendarDays size={20} />
        <div>
          <strong>Juste ce que votre chaudière sait</strong>
          <p>
            Les détails matin, midi et soir apparaissent uniquement si la source
            fournit des horaires explicites. Une prévision absente reste
            indiquée comme indisponible.
          </p>
        </div>
      </div>
    </>
  );
}
