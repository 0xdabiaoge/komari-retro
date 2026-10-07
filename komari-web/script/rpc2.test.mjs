import fs from 'node:fs';
import ts from 'typescript';
import { test } from 'node:test';
import assert from 'node:assert/strict';
let source = fs.readFileSync(new URL('../src/lib/rpc2.ts', import.meta.url), 'utf8');
source = source.replace('import { RPC2ConnectionState } from "../types/rpc2";', 'const RPC2ConnectionState = { CONNECTED:"connected", DISCONNECTED:"disconnected", CONNECTING:"connecting", ERROR:"error", RECONNECTING:"reconnecting" };');
source = source.replace('import i18n from "../i18n/config";', 'const i18n = { t: s => s };');
const code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 } }).outputText;
const { RPC2Client } = await import('data:text/javascript;base64,' + Buffer.from(code).toString('base64'));

function simulatedClient() {
 const client=new RPC2Client('/api/rpc2',{autoConnect:false});client.connectionState='connected';return client;
}
test('lost mutation response must never execute through HTTP again',async()=>{
 const client=simulatedClient();let writes=0;
 client.callViaWebSocket=async()=>{writes++;throw new Error('response lost')};
 client.callViaHTTP=async()=>{writes++;return 'ok'};
 await assert.rejects(client.call('admin:exec',{command:'simulation'}),/response lost/);
 assert.equal(writes,1);
});
test('read requests can recover over HTTP',async()=>{
 const client=simulatedClient();let reads=0;
 client.callViaWebSocket=async()=>{reads++;throw new Error('response lost')};
 client.callViaHTTP=async()=>{reads++;return 'ok'};
 assert.equal(await client.call('common:getNodes'), 'ok');assert.equal(reads,2);
});
