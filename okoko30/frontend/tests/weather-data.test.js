import test from 'node:test';
import assert from 'node:assert/strict';
import { forecastNumber, parseForecastDate } from '../src/weather-data.js';
test('absent and malformed weather values remain unknown',()=>{for(const input of [undefined,null,'','   ','not-a-number'])assert.equal(forecastNumber(input),null);assert.equal(forecastNumber('0'),0);assert.equal(forecastNumber('12,5'),12.5)});
test('invalid calendar dates and times are not reassigned to another day',()=>{for(const raw of ['31.02.2026','2026-13-01','2026-10-22 25:00','2026-10-22 10:75','demain'])assert.equal(parseForecastDate(raw),null)});
test('a daily forecast has no invented hourly slot',()=>{const day=parseForecastDate('22.10.2026');assert.equal(day.hourly,false);assert.equal(day.date.getDate(),22);assert.equal(day.date.getMonth(),9)});
test('an explicit midnight remains midnight',()=>{const entry=parseForecastDate('2026-10-22 00:00');assert.equal(entry.hourly,true);assert.equal(entry.date.getHours(),0)});

test('wind units are shown once and missing wind stays unknown', async()=>{
 const { windLabel } = await import('../src/weather-data.js');
 assert.equal(windLabel('57 km/h'),'57 km/h');
 assert.equal(windLabel('57'),'57 km/h');
 assert.equal(windLabel(''),'—');
 assert.equal(windLabel(null),'—');
});
