"""Bounded adversarial checks for disposable loopback Komari fixtures only.
Use the private fixture runner, never a production database. Real provider
credentials are neither accepted nor needed by this script.
"""
import argparse,base64,concurrent.futures,hashlib,http.client,io,json,os,re,secrets,socket,sqlite3,struct,subprocess,time,urllib.error,urllib.parse,urllib.request,zipfile
from pathlib import Path
parser=argparse.ArgumentParser();parser.add_argument('--base',required=True);parser.add_argument('--data-dir',required=True);parser.add_argument('--output',required=True);parser.add_argument('--agent-image',required=True);parser.add_argument('--methods',required=True);parser.add_argument('--container',required=True);args=parser.parse_args()
base=args.base.rstrip('/');parsed=urllib.parse.urlparse(base)
assert parsed.hostname=='127.0.0.1' and 25900<=parsed.port<26000,'Disposable loopback fixture required'
data=Path(args.data_dir);assert (data.parent/'.security-fixture').is_file(),'Fixture marker required'
results=[]
def check(name,fn):
 start=time.monotonic()
 try:fn();row={'case':name,'status':'PASS'}
 except Exception as error:row={'case':name,'status':'FAIL','error':str(error)[:180]}
 row['seconds']=round(time.monotonic()-start,3);results.append(row)
 Path(args.output).write_text(json.dumps(results,indent=2));print(name,row['status'],row.get('error',''),flush=True)
 return row['status']=='PASS'
def request(path,body=None,headers=None,method=None):
 h={'Content-Type':'application/json'};h.update(headers or {})
 if isinstance(body,(dict,list)):body=json.dumps(body).encode()
 req=urllib.request.Request(base+path,data=body,headers=h,method=method)
 try:r=urllib.request.urlopen(req,timeout=22)
 except urllib.error.HTTPError as error:r=error
 return r.status,r.read(),r.headers
creds=dict(line.split(': ',1) for line in (data/'initial-admin.txt').read_text().strip().splitlines())
status,body,headers=request('/api/login',{'username':creds['Username'],'password':creds['Password']});assert status==200
session=next(c.split(';')[0].split('=',1)[1] for c in headers.get_all('Set-Cookie') if c.startswith('session_token='));admin={'Cookie':'session_token='+session}
def rpc(method,params=None,headers=admin):
 status,body,_=request('/api/rpc2',{'jsonrpc':'2.0','id':1,'method':method,'params':params or {}},headers)
 assert status==200,'RPC transport failure '+str(status)
 return json.loads(body)
def success(method,params=None):
 value=rpc(method,params);assert 'error' not in value,method+' was refused';return value.get('result')
def expect(condition,message='unexpected authorization or response'):
 assert condition,message
perm=secrets.token_urlsafe(24);temp=secrets.token_urlsafe(24);entrance='/entry-'+secrets.token_hex(16);view='/view-'+secrets.token_hex(16);canary='PRIVATE-'+secrets.token_hex(12)
node=success('admin:addClient',{'name':'Security-Public'});hidden=success('admin:addClient',{'name':canary})
success('admin:editClient',{'uuid':hidden['uuid'],'hidden':True,'remark':canary,'ipv4':'198.51.100.17'})
success('admin:editSettings',{'private_site':True,'admin_path':entrance,'admin_view_path':view,'permanent_share_token':perm,'tempory_share_token':temp,'tempory_share_token_expire_at':int(time.time())+120,'send_ip_addr_to_guest':False,'notification_enabled':False,'login_notification':False,'cors_origin_check_enabled':True})
share={'Cookie':'temp_key='+perm};agent={'Authorization':'Bearer '+node['token']}
readonly='readonly-'+secrets.token_hex(16);success('admin:editSettings',{'api_key':readonly,'api_key_scope':'read-only'})
roles={'anonymous':{},'share':share,'read-only-key':{'Authorization':'Bearer '+readonly},'agent-token':agent}
methods=json.loads(Path(args.methods).read_text())
class WS:
 def __init__(self,path='/api/rpc2',headers=None,origin=None):
  self.s=socket.create_connection(('127.0.0.1',parsed.port),timeout=5);self.s.settimeout(8)
  key=base64.b64encode(os.urandom(16)).decode();h={'Host':parsed.netloc,'Origin':origin or base,'Connection':'Upgrade','Upgrade':'websocket','Sec-WebSocket-Version':'13','Sec-WebSocket-Key':key};h.update(headers or {})
  self.s.sendall(('GET '+path+' HTTP/1.1\r\n'+''.join(k+': '+v+'\r\n' for k,v in h.items())+'\r\n').encode())
  raw=b''
  while not raw.endswith(b'\r\n\r\n'):
   part=self.s.recv(1)
   if not part:break
   raw+=part
  self.status=int(raw.split(b' ')[1])
  if self.status!=101:self.s.close()
 def send(self,value):
  payload=json.dumps(value).encode() if isinstance(value,dict) else value.encode() if isinstance(value,str) else value
  n=len(payload);mask=os.urandom(4);length=bytes([128|n]) if n<126 else bytes([254])+struct.pack('!H',n) if n<65536 else bytes([255])+struct.pack('!Q',n)
  self.s.sendall(bytes([129])+length+mask+bytes(b^mask[i%4] for i,b in enumerate(payload)))
 def readn(self,n):
  data=b''
  while len(data)<n:
   part=self.s.recv(n-len(data))
   if not part:raise EOFError('closed')
   data+=part
  return data
 def recv(self):
  while True:
   a,b=self.readn(2);n=b&127
   if n==126:n=struct.unpack('!H',self.readn(2))[0]
   if n==127:n=struct.unpack('!Q',self.readn(8))[0]
   data=self.readn(n)
   if a&15==8:raise EOFError('closed')
   if a&15==9:continue
   return data
 def close(self):self.s.close()
def websocket_boundaries():
 bad=WS(headers=admin,origin='https://evil.invalid');expect(bad.status==403,'cross-origin WS accepted')
 ws=WS(headers=share)
 try:
  expect(ws.status==101);ws.send({'jsonrpc':'2.0','id':1,'method':'admin:getSettings'});expect(json.loads(ws.recv()).get('error',{}).get('code')==-32041)
  ws.send(b'{')
  try:expect('error' in json.loads(ws.recv()))
  except (EOFError,OSError):pass
  ws.close();ws=WS(headers=share);expect(ws.status==101)
  ws.send({'jsonrpc':'2.0','id':2,'method':'rpc.ping'});expect(json.loads(ws.recv()).get('result')=='pong')
  try:
   ws.send(b'x'*((1<<20)+1));ws.recv()
   raise AssertionError('oversized WS frame remained readable')
  except socket.timeout:raise AssertionError('oversized WS frame was not promptly closed')
  except (OSError,EOFError):pass
 finally:ws.close()
def issue_session():
 st,_,h=request('/api/login',{'username':creds['Username'],'password':creds['Password']},{'Cookie':'admin_entrance_token='+entrance});expect(st==200)
 return next(c.split(';')[0].split('=',1)[1] for c in h.get_all('Set-Cookie') if c.startswith('session_token='))
def websocket_revocation():
 token=issue_session();ws=WS(headers={'Cookie':'session_token='+token})
 try:
  expect(ws.status==101);success('admin:deleteSession',{'session':token});ws.send({'jsonrpc':'2.0','id':1,'method':'admin:getSettings'});expect(json.loads(ws.recv()).get('error',{}).get('code')==-32041)
 finally:ws.close()

for role,h in roles.items():
 for method in methods:
  check('acl/'+role+'/'+method,lambda method=method,h=h:expect(rpc(method,headers=h).get('error',{}).get('code')==-32041,'privileged method reached without full admin rights'))
for path in ['/','/admin','/admin/login','/terminal','/api/public','/api/nodes','/api/admin/settings','/api/admin/download/backup','/.git/config','/data/komari.db','/data/initial-admin.txt','/api/oauth','/api/oauth_callback']:
 check('private/anonymous'+path,lambda path=path:expect(request(path,headers={})[0] in [401,403,404]))
for path in ['/api/oauth','/api/oauth_callback','/api/admin/settings/oidc','/api/admin/oauth2/bind','/api/admin/2fa/generate']:
 check('removed'+path,lambda path=path:expect(request(path,headers=admin)[0]==404))
check('removed/oidc-rpc',lambda:expect(rpc('admin:getOidcProvider').get('error',{}).get('code')==-32601))
def removed_password_toggle():
 result=rpc('admin:editSettings',{'disable_password_login':True})
 if 'error' not in result:success('admin:editSettings',{'disable_password_login':False})
 expect(result.get('error',{}).get('code')==-32602)
check('removed/password-disable-setting-rejected',removed_password_toggle)
for h in [{},{'X-Forwarded-For':'127.0.0.1','X-Real-IP':'127.0.0.1'},{'Cookie':'session_token=forged;admin_entrance_token=forged'},{'Authorization':'Bearer forged'},{'Authorization':'Bearer '+node['token']}]:
 check('auth-forgery/'+str(len(results)),lambda h=h:expect('error' in rpc('admin:getSettings',headers=h)))
for origin in ['https://evil.invalid','null','https://127.0.0.1.evil.invalid','https://user@evil.invalid']:
 check('cors/'+origin,lambda origin=origin:expect(request('/api/rpc2',{'jsonrpc':'2.0','id':1,'method':'admin:getSettings'},{**admin,'Origin':origin})[0]==403))
check('csrf/theme-get-no-mutation',lambda:expect(request('/api/admin/theme/set?theme=retro',headers={**admin,'Sec-Fetch-Site':'cross-site'})[0] in [404,405]))
def logout_csrf():
 token=issue_session()
 try:expect(request('/api/logout',headers={'Cookie':'session_token='+token,'Sec-Fetch-Site':'cross-site'})[0]==403)
 finally:success('admin:deleteSession',{'session':token})
check('csrf/logout-cross-site-denied',logout_csrf)
check('csrf/same-origin-theme-post',lambda:expect(request('/api/admin/theme/set?theme=default',{},admin)[0]==200))
for h in [share,{'X-Temp-Key':perm}]:
 def redaction(h=h):
  for path in ['/api/nodes','/api/public']:
   status,body,_=request(path,headers=h);expect(status==200)
   for secret in [node['token'],hidden['token'],hidden['uuid'],canary,entrance,view,perm]:expect(secret.encode() not in body,'private data leaked')
 check('share/redaction-'+str(len(results)),redaction)
check('share/permanent-page-and-cookie',lambda:expect(request('/s/'+perm,headers={})[0]==200))
check('share/temporary-valid',lambda:expect(request('/api/public',headers={'Cookie':'temp_key='+temp})[0]==200))
success('admin:editSettings',{'tempory_share_token_expire_at':int(time.time())-1})
check('share/temporary-expired',lambda:expect(request('/api/public',headers={'Cookie':'temp_key='+temp})[0]==404))
check('share/forged',lambda:expect(request('/api/public',headers={'Cookie':'temp_key=forged'})[0]==404))
check('websocket/origin-malformed-recovery-frame-limit',websocket_boundaries)
check('websocket/revoked-admin-session',websocket_revocation)
check('headers/no-referrer',lambda:expect(request('/s/'+perm,headers={})[2].get('Referrer-Policy')=='no-referrer'))
for suffix in ['/file/download?path=/etc/shadow','/file/preview-token?path=/etc/shadow','/terminal']:
 check('files/guest-denied'+suffix,lambda suffix=suffix:expect(request('/api/admin/client/'+node['uuid']+suffix,headers=share)[0] in [401,403,404]))
check('files/forged-preview-denied',lambda:expect(request('/api/preview/client/'+node['uuid']+'/file/download?path=/etc/shadow&token=forged',headers={})[0] in [400,401,403,404]))
check('files/unknown-transfer-denied',lambda:expect(request('/api/clients/transfer/forged?transfer_token=forged',b'bytes',agent)[0] in [400,401,403,404]))
for url in ['http://127.0.0.1:1/theme.zip','http://169.254.169.254/latest/meta-data','file:///etc/passwd','http://[::1]/theme.zip']:
 check('ssrf/'+url,lambda url=url:expect(request('/api/admin/theme/import',{'url':url},admin)[0]>=400))
def traversal_archive():
 content=io.BytesIO()
 with zipfile.ZipFile(content,'w') as z:
  z.writestr('komari-theme.json',json.dumps({'name':'Fixture','short':'fixture','version':'1'}));z.writestr('dist/index.html','fixture');z.writestr('../../outside-secret','no')
 status,body,_=request('/api/admin/theme/upload',content.getvalue(),{**admin,'Content-Type':'application/zip'},'PUT')
 expect(status==400 and any(word in body.lower() for word in [b'path',b'travers',b'escape']), 'traversal was not rejected by the archive validator')
check('archive/traversal',traversal_archive)
for injected in ["' OR 1=1 --",'../..','%00','<img src=x onerror=alert(1)>']:
 check('input/query-'+str(len(results)),lambda injected=injected:expect(request('/api/admin/client/list?name='+urllib.parse.quote(injected),headers=admin)[0] in [200,400]))
for metric in ["cpu);DROP TABLE clients;--",'unknown','../token']:
 check('input/metric-'+str(len(results)),lambda metric=metric:expect('error' in rpc('public:queryMetrics',{'metric_keys':[metric]},share)))
for payload in [b'',b'{',b'[]',b'null',b'1',b'"x"',b'['*10001+b'0'+b']'*10001,b'{}',b'{"jsonrpc":"2.0","method":1}',bytes([255,254])]:
 check('json/malformed-'+str(len(results)),lambda payload=payload:expect(request('/api/rpc2',payload,share)[0] in [400,413]))
large_batch=[{'jsonrpc':'2.0','id':i,'method':'rpc.ping'} for i in range(101)]
check('rpc/batch-cap',lambda:expect(request('/api/rpc2',large_batch,share)[0]==413))
def oversized_body_headers():
 s=socket.create_connection(('127.0.0.1',parsed.port),timeout=3);s.settimeout(3)
 try:
  s.sendall(('POST /api/rpc2 HTTP/1.1\r\nHost: '+parsed.netloc+'\r\nContent-Type: application/json\r\nContent-Length: 3145728\r\n\r\n').encode());expect(b' 413 ' in s.recv(256))
 finally:s.close()
check('http/body-size-cap',oversized_body_headers)
# Send only headers. A server buffering the anonymous body before auth will
# stall here; a safe rejection returns before any bulk upload is sent.
for route in ['/api/admin/upload/backup','/api/admin/theme/upload','/api/clients/transfer/forged']:
 def reject_without_body(route=route):
  s=socket.create_connection(('127.0.0.1',parsed.port),timeout=3);s.settimeout(3)
  try:
   s.sendall(('POST '+route+' HTTP/1.1\r\nHost: 127.0.0.1:'+str(parsed.port)+'\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n').encode())
   response=s.recv(256);expect(any(code in response for code in [b' 401 ',b' 403 ',b' 404 ']),'anonymous upload was read before authorization')
  finally:s.close()
 check('resource/anonymous-upload-early-reject'+route,reject_without_body)
# A bounded burst tests real HTTP parsing, dispatch and SQLite contention.
def burst():
 def one(i):return request('/api/rpc2',{'jsonrpc':'2.0','id':i,'method':'public:queryMetrics','params':{'metric_keys':['cpu.usage'],'hours':1,'max_points':10000}},share)[0]
 with concurrent.futures.ThreadPoolExecutor(max_workers=32) as pool:codes=list(pool.map(one,range(160)))
 expect(all(c in [200,429,503] for c in codes),'unexpected burst failures');expect(request('/ping')[1]==b'pong')
check('resource/32-concurrent-160-metric-queries',burst)
def connection_budget():
 conns=[]
 try:
  for _ in range(290):
   try:
    s=socket.create_connection(('127.0.0.1',parsed.port),timeout=.15);s.sendall(b'GET /ping HTTP/1.1\r\nHost: fixture\r\n');conns.append(s)
   except OSError:pass
  pid=subprocess.check_output(['docker','inspect','-f','{{.State.Pid}}',args.container],text=True).strip()
  expect(len(list((Path('/proc')/pid/'fd').iterdir()))<=280,'socket/FD quota was exceeded')
 finally:
  for s in conns:s.close()
 for _ in range(15):
  try:
   if request('/ping')[1]==b'pong':return
  except OSError:time.sleep(.2)
 raise AssertionError('server did not recover after connection pressure')
check('resource/290-slow-header-connections-and-recovery',connection_budget)

def slow_json():
 s=socket.create_connection(('127.0.0.1',parsed.port),timeout=3);s.settimeout(19);started=time.monotonic()
 try:
  s.sendall(('POST /api/rpc2 HTTP/1.1\r\nHost: 127.0.0.1:'+str(parsed.port)+'\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{').encode());s.recv(1024)
  expect(time.monotonic()-started<18,'slow JSON body did not time out')
 finally:s.close()
check('resource/slow-json-read-deadline',slow_json)
def json_memory_burst():
 payload=b'{"padding":"'+b'x'*(1<<20)+b'"}'
 def one(i):return request('/api/rpc2',payload,share)[0]
 with concurrent.futures.ThreadPoolExecutor(max_workers=24) as pool:codes=list(pool.map(one,range(64)))
 expect(all(code in [400,413,429,503] for code in codes));expect(request('/ping')[1]==b'pong')
check('resource/24-concurrent-64-oversized-json-bodies',json_memory_burst)

check('health/after-adversarial-requests',lambda:expect(request('/ping')[1]==b'pong'))
check('data/node-inventory-preserved',lambda:expect(len(success('admin:listClients'))==2))
check('data/sqlite-integrity',lambda:expect(sqlite3.connect('file:'+str(data/'komari.db')+'?mode=ro',uri=True).execute('pragma integrity_check').fetchone()[0]=='ok'))
def terminal_revocation_and_quota():
 name='komari-security-agent-'+str(parsed.port)
 command=['docker','run','-d','--name',name,'--network','host','--memory','128m','--memory-swap','128m','--cpus','0.25','--pids-limit','64','--cap-drop','ALL','--security-opt','no-new-privileges','--user','65534:65534','--read-only','--tmpfs','/tmp:rw,nosuid,size=32m',args.agent_image,'-e',base,'-t',node['token'],'--disable-auto-update','--get-ip-addr-from-nic','--interval','1']
 process=subprocess.run(command,capture_output=True,text=True);expect(process.returncode==0,'isolated Agent startup failed');connections=[]
 try:
  for _ in range(80):
   if success('common:getNodesLatestStatus').get(node['uuid'],{}).get('online'):break
   time.sleep(.25)
  else:raise AssertionError('isolated Agent did not connect')
  token=issue_session();ws=WS('/api/admin/client/'+node['uuid']+'/terminal',{'Cookie':'session_token='+token});connections.append(ws);expect(ws.status==101)
  time.sleep(.5);ws.send('printf "SECURITY_%s\\n" "READY"\n')
  received=b'';deadline=time.monotonic()+10;ws.s.settimeout(1)
  while b'SECURITY_READY' not in received and time.monotonic()<deadline:
   try:received+=ws.recv()
   except socket.timeout:continue
   expect(len(received)<65536,'unexpected terminal output volume')
  if b'SECURITY_READY' not in received:
   Path(args.output).with_name('terminal-fixture-output.bin').write_bytes(received)
   raise AssertionError('real PTY did not execute fixture command')
  ws.s.settimeout(8)
  success('admin:deleteSession',{'session':token});started=time.monotonic();closed=False
  try:
   while time.monotonic()-started<6:ws.recv()
  except socket.timeout:closed=False
  except (EOFError,OSError):closed=True
  expect(closed,'revoked browser terminal remained connected')
  time.sleep(1)
  for i in range(8):
   conn=WS('/api/admin/client/'+node['uuid']+'/terminal',admin);expect(conn.status==101);connections.append(conn);conn.recv()
  extra=WS('/api/admin/client/'+node['uuid']+'/terminal',admin);connections.append(extra)
  expect(extra.status==101 and b'capacity' in extra.recv().lower(),'terminal quota was not enforced')
 finally:
  for conn in connections:conn.close()
  subprocess.run(['docker','rm','-f',name],capture_output=True)
check('terminal/real-pty-revocation-and-session-quota',terminal_revocation_and_quota)

# Brute-force counters and bcrypt concurrency are tested last to avoid locking
# fixture logins used by earlier cases.
def login_burst():
 def one(i):return request('/api/login',{'username':creds['Username'],'password':'wrong'}, {'Cookie':'admin_entrance_token='+entrance,'X-Forwarded-For':'203.0.113.'+str(i%200+1)})[0]
 with concurrent.futures.ThreadPoolExecutor(max_workers=24) as pool:codes=list(pool.map(one,range(48)))
 expect(429 in codes,'no concurrent/rate admission rejection');expect(all(c in [401,429,503] for c in codes));expect(request('/ping')[1]==b'pong')
check('resource/parallel-login-and-forged-ip-limits',login_burst)
print('SECURITY_TOTAL',len(results),'PASS',sum(r['status']=='PASS' for r in results),'FAIL',sum(r['status']=='FAIL' for r in results),flush=True)
raise SystemExit(1 if any(r['status']=='FAIL' for r in results) else 0)
