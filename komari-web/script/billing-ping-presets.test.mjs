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
test('billing offers exactly five cycles and never silently shortens legacy periods',()=>{
 assert.deepEqual(billing.BILLING_CYCLES.map(p=>p.label),['天','月','季','年','免费/一次性']);
 assert.deepEqual(billing.BILLING_CYCLES.map(p=>p.value),['1','30','92','365','-1']);
 for(const [days,value] of [[1,'1'],[0,'30'],[30,'30'],[90,'92'],[92,'92'],[366,'365'],[-1,'-1'],[7,''],[184,''],[730,'']])assert.equal(billing.billingCycleOption(days),value);
});

test('region cards expose both IP families and selection stays independent across groups',()=>{
 const catalog=JSON.parse(fs.readFileSync(new URL('../../pkg/pingpresets/catalog.json',import.meta.url),'utf8')).entries;
 const mainland=presets.groupPingPresets(catalog,'mainland');assert.equal(mainland.length,31);
 assert.equal(mainland.flatMap(r=>r.targets).length,186);
 const beijing=mainland.find(r=>r.region==='北京');assert.deepEqual(beijing.families.map(f=>[f.family,f.targets.length]),[['ipv4',3],['ipv6',3]]);
 assert.equal(presets.groupPingPresets(catalog,'global').flatMap(r=>r.targets).length,4);
 assert.equal(presets.groupPingPresets(catalog,'overseas').length,11);
 assert.deepEqual(presets.togglePingPresetSelection(['hidden','a'],['a','b'],true),['hidden','a','b']);assert.deepEqual(presets.togglePingPresetSelection(['hidden','a','b'],['a','b'],false),['hidden']);
});

test('custom probe edits validate name, address and protocol before import',()=>{
 const make=(type,target,name=' 自定义探测 ')=>({id:'global-google-ipv4',name,type,target});
 for(const [type,target] of [['icmp','8.8.8.8'],['icmp','2001:4860:4860::8888'],['icmp','example.com'],['tcp','example.com:80'],['tcp','[2001:db8::1]:443'],['http','https://example.com/check?q=1']])assert.equal(presets.normalizePingPresetEdit(make(type,target)).name,'自定义探测');
 for(const [type,target] of [['tcp','example.com'],['tcp','[::1]:65536'],['icmp','https://example.com'],['icmp','example.com:80'],['icmp','8.8.8.8\nother'],['http','ftp://example.com'],['http','https://user:pass@example.com']])assert.throws(()=>presets.normalizePingPresetEdit(make(type,target)));
 assert.throws(()=>presets.normalizePingPresetEdit(make('icmp','8.8.8.8',' ')));
 assert.throws(()=>presets.normalizePingPresetEdit(make('icmp','8.8.8.8','a'.repeat(129))));
 assert.equal(presets.pingPresetTargetKey('http','https://EXAMPLE.com/Status?key=A'),'http|https://example.com/Status?key=A');
 assert.notEqual(presets.pingPresetTargetKey('http','https://example.com/Status'),presets.pingPresetTargetKey('http','https://example.com/status'));
});
