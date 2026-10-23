"""Disposable local server + native Agent acceptance. Never connects to production."""
import http.cookiejar,json,os,pathlib,secrets,socket,sqlite3,subprocess,sys,tempfile,time,urllib.request
root=pathlib.Path(__file__).resolve().parents[1]
server=pathlib.Path(sys.argv[1]).resolve();agent=pathlib.Path(sys.argv[2]).resolve()
(root/'.codex/build').mkdir(parents=True,exist_ok=True)
work=pathlib.Path(tempfile.mkdtemp(prefix='terminal-',dir=root/'.codex/build'));data=work/'data';data.mkdir()
with socket.socket() as probe:probe.bind(('127.0.0.1',0));port=probe.getsockname()[1]
base='http://127.0.0.1:'+str(port);entry='/entry-'+secrets.token_hex(24)
db=sqlite3.connect(data/'komari.db');db.execute('create table configs(key text primary key,value text)')
for k,v in {'eula_accepted':True,'notification_enabled':False,'login_notification':False,'geo_ip_enabled':False,'private_site':True,'admin_path':entry,'theme_settings':{'_komari_onboarding_v1':{'seen':['install','workbench','notifications','markets','terminal'],'workbenchOpened':True}}}.items():db.execute('insert into configs values(?,?)',(k,json.dumps(v)))
db.commit();db.close();processes=[];logs=[]
def start(args,env,name):
 log=(work/name).open('w',encoding='utf-8');logs.append(log);p=subprocess.Popen(args,cwd=work,env=env,stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0);processes.append(p);return p
try:
 env={k:v for k,v in os.environ.items() if not k.startswith(('KOMARI_','AGENT_'))};env.update(KOMARI_DB_TYPE='sqlite',KOMARI_DB_FILE=str(data/'komari.db'),KOMARI_LISTEN='127.0.0.1:'+str(port),KOMARI_CLOUDFLARED_TOKEN='')
 start([str(server),'server'],env,'server.log')
 for _ in range(120):
  try:
   if urllib.request.urlopen(base+'/ping',timeout=1).read()==b'pong':break
  except OSError:pass
  time.sleep(.25)
 else:raise RuntimeError('local server startup timeout')
 creds=dict(x.split(': ',1) for x in (data/'initial-admin.txt').read_text().strip().splitlines())
 jar=http.cookiejar.CookieJar();opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar));opener.open(base+entry).read()
 opener.open(urllib.request.Request(base+'/api/login',data=json.dumps({'username':creds['Username'],'password':creds['Password']}).encode(),headers={'Content-Type':'application/json'})).read()
 def rpc(method,params={}):
  value=json.load(opener.open(urllib.request.Request(base+'/api/rpc2',data=json.dumps({'jsonrpc':'2.0','id':1,'method':method,'params':params}).encode(),headers={'Content-Type':'application/json'})));assert 'error' not in value,method;return value['result']
 node=rpc('admin:addClient',{'name':'Native terminal acceptance'})
 theme={'_komari_onboarding_v1':{'seen':['install','workbench','notifications','markets','terminal'],'workbenchOpened':True}}
 opener.open(urllib.request.Request(base+'/api/admin/theme/settings?theme=default',data=json.dumps(theme).encode(),headers={'Content-Type':'application/json'})).read()
 ae={**env,'AGENT_ENDPOINT':base,'AGENT_TOKEN':node['token'],'AGENT_DISABLE_AUTO_UPDATE':'true','AGENT_CUSTOM_IPV4':'127.0.0.1','AGENT_CUSTOM_IPV6':'::1','AGENT_INTERVAL':'1','AGENT_RECONNECT_INTERVAL':'1'}
 if os.name=='nt':ae['PSModulePath']=os.path.join(os.environ['SystemRoot'],'System32','WindowsPowerShell','v1.0','Modules')
 start([str(agent)],ae,'agent.log')
 for _ in range(100):
  states=rpc('common:getNodesLatestStatus')
  if node['uuid'] in states and states[node['uuid']].get('online'):break
  time.sleep(.3)
 else:raise RuntimeError('native Agent did not connect')
 info=work/'fixture.json';info.write_text(json.dumps({'base':base,'entry':entry,'username':creds['Username'],'password':creds['Password'],'node':node['uuid'],'windows':os.name=='nt'}))
 result=subprocess.run(['node',str(root/'komari-web/script/terminal-real.browser.mjs')],cwd=root/'komari-web',env={**os.environ,'TERMINAL_REAL_FIXTURE':str(info)},timeout=180);assert result.returncode==0,'real browser terminal acceptance failed'
 print('PASS: disposable real server / native Agent / PTY / browser / reattachment / permission revocation',flush=True)
finally:
 for p in reversed(processes):
  p.terminate()
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait(timeout=5)
 for log in logs:log.close()
 # Fixture credentials stay local, never printed or committed.
 for p in [work/'fixture.json',data/'initial-admin.txt']:
  if p.exists():p.unlink()
