import fs from 'node:fs';
import ts from 'typescript';
import { test } from 'node:test';
import assert from 'node:assert/strict';
let source=fs.readFileSync(new URL('../src/lib/backupUpload.ts',import.meta.url),'utf8').replace('import { authorizeSensitiveAccess } from "./sensitive";','const authorizeSensitiveAccess=async()=>{};');
const code=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ES2022,target:ts.ScriptTarget.ES2022}}).outputText;
const {createBackupUploadTask}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));

test('backup uses actual multipart endpoint with progress and no mutation retry',async()=>{
 const original=globalThis.XMLHttpRequest;let requests=0;
 class FakeXHR extends EventTarget {
  upload=new EventTarget();status=200;responseText='{"status":"success"}';
  open(method,url){assert.equal(method,'POST');assert.equal(url,'/api/admin/upload/backup');requests++;}
  send(body){assert.equal(body.get('backup').name,'backup.zip');queueMicrotask(()=>this.dispatchEvent(new Event('load')));}
 }
 globalThis.XMLHttpRequest=FakeXHR;
 try {const progress=[];await createBackupUploadTask().upload('backup',new File(['zip-fixture'],'backup.zip'),value=>progress.push(value));assert.deepEqual(progress,[100]);assert.equal(requests,1);}
 finally {globalThis.XMLHttpRequest=original;}
});
test('cancellation before request prevents sending a backup',async()=>{
 const task=createBackupUploadTask();task.cancel();await assert.rejects(task.upload('backup',new File(['zip-fixture'],'backup.zip'),()=>{}),{name:'AbortError'});
});
