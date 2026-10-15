import fs from 'node:fs';
import ts from 'typescript';
import {test} from 'node:test';
import assert from 'node:assert/strict';
const currencies=JSON.parse(fs.readFileSync(new URL('../../shared/currencies.json',import.meta.url),'utf8'));
async function load(path,replace){
 let source=fs.readFileSync(new URL(path,import.meta.url),'utf8');if(replace)source=source.replace(/import currencies from [^;]+;/,`const currencies=${JSON.stringify(currencies)};`);
 const code=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
 return import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
}
const billing=await load('../../shared/billing.ts',true),presets=await load('../src/utils/pingPresets.ts');
test('price modes preserve hidden/free sentinels and reject invalid paid amounts',()=>{
 assert.equal(billing.billingPrice('hidden','100'),0);assert.equal(billing.billingPrice('free','100'),-1);assert.equal(billing.billingPrice('paid','19.99'),19.99);
 for(const amount of ['', '0', '-1','NaN','Infinity','12oops'])assert.throws(()=>billing.billingPrice('paid',amount));
 assert.equal(billing.priceModeFromPrice(-1),'free');assert.equal(billing.priceModeFromPrice(0),'hidden');assert.equal(billing.priceModeFromPrice(1),'paid');
});
test('canonical currencies distinguish yen/yuan and preserve existing unknown values',()=>{
 assert.equal(billing.normalizeBillingCurrency('￥'),'CNY');assert.equal(billing.normalizeBillingCurrency('JPY'),'JPY');assert.equal(billing.normalizeBillingCurrency('$'),'USD');assert.equal(billing.normalizeBillingCurrency('HK$'),'HKD');
 assert.equal(billing.formatBillingAmount('JPY',500),'JP¥500');assert.equal(billing.formatBillingAmount('CNY',500),'¥500');assert.equal(billing.formatBillingAmount('$',5),'$5');assert.equal(billing.normalizeBillingCurrency('legacy-unit'),'legacy-unit');
 assert.equal(new Set(currencies.map(c=>c.code)).size,currencies.length);
});
test('preset filters and selection retain hidden selections and never duplicate ids',()=>{
 const catalog=JSON.parse(fs.readFileSync(new URL('../../pkg/pingpresets/catalog.json',import.meta.url),'utf8')).entries;
 assert.equal(presets.filterPingPresets(catalog,'mainland','ipv4','all','').length,93);
 assert.equal(presets.filterPingPresets(catalog,'mainland','ipv6','all','').length,93);
 assert.equal(presets.filterPingPresets(catalog,'mainland','all','电信','北京').length,2);
 assert.equal(presets.filterPingPresets(catalog,'global','ipv4','all','').length,2);
 assert.equal(presets.filterPingPresets(catalog,'overseas','ipv4','all','').length,11);
 assert.deepEqual(presets.togglePingPresetSelection(['hidden','a'],['a','b'],true),['hidden','a','b']);assert.deepEqual(presets.togglePingPresetSelection(['hidden','a','b'],['a','b'],false),['hidden']);
});
