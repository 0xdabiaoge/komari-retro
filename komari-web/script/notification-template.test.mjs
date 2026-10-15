import fs from 'node:fs';
import ts from 'typescript';
import {test} from 'node:test';
import assert from 'node:assert/strict';
const catalog=JSON.parse(fs.readFileSync(new URL('../../pkg/notification/catalog.json',import.meta.url),'utf8'));
const source=fs.readFileSync(new URL('../src/utils/notificationTemplate.ts',import.meta.url),'utf8').replace(/import catalog from [^;]+;/,`const catalog=${JSON.stringify(catalog)};`);
const code=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
const {substituteNotificationTemplate,notificationSampleData}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
test('notification values are escaped once and never recursively expanded',()=>{
 const data={message:'<img src=x onerror=alert(1)> & "quoted" {{time}}',time:'SHOULD_NOT_REPLACE_DATA'};
 assert.equal(substituteNotificationTemplate('{{message}}|{{ message }}',data),'&lt;img src=x onerror=alert(1)&gt; &amp; &#34;quoted&#34; {{time}}|&lt;img src=x onerror=alert(1)&gt; &amp; &#34;quoted&#34; {{time}}');
});
test('aliases preserve old templates and unknown variables stay visible',()=>{
 assert.equal(substituteNotificationTemplate('{{client_name}} {{client_ip}} {{server}} {{status_text}} {{missing}}',{client:'A',ip:'B',site_name:'C',status:'D'}),'A B C D {{missing}}');
});
test('every shared preset variable is present in single and multi node samples',()=>{
 for(const batch of [false,true]){
  const data=notificationSampleData('offline',batch);
  for(const preset of catalog.presets)assert.ok(!substituteNotificationTemplate(preset.template,data).includes('{{'),preset.id);
  for(const variable of catalog.variables)assert.ok(data[variable.key]!==undefined,variable.key);
  assert.equal(data.client_count,batch?'2':'1');
 }
});
