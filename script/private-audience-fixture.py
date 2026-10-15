"""Run real browser acceptance against a disposable loopback server, never production."""
import http.cookiejar,json,os,pathlib,secrets,socket,sqlite3,subprocess,sys,tempfile,time,urllib.request

root=pathlib.Path(__file__).resolve().parents[1]
binary=pathlib.Path(sys.argv[1]).resolve()
assert binary.is_file()
base_dir=root/'.codex'/'audience-fixtures';base_dir.mkdir(parents=True,exist_ok=True)
work=pathlib.Path(tempfile.mkdtemp(prefix='fixture-',dir=base_dir)).resolve()
assert work.is_relative_to(base_dir.resolve())
os.chmod(work,0o700);data=work/'data';data.mkdir()
port=None
for candidate in range(25980,25990):
 try:
  with socket.socket() as probe:probe.bind(('127.0.0.1',candidate))
  port=candidate;break
 except OSError:pass
assert port,'no free fixture port'
base='http://127.0.0.1:'+str(port)
entry='/entry-'+secrets.token_hex(24);view='/view-'+secrets.token_hex(24)
share=secrets.token_urlsafe(32);temporary=secrets.token_urlsafe(32)
db=sqlite3.connect(data/'komari.db');db.execute('create table configs(key text primary key,value text)')
for key,value in {'eula_accepted':True,'notification_enabled':False,'login_notification':False,'geo_ip_enabled':False,'private_site':True,'admin_path':entry,'admin_view_path':view,'permanent_share_token':share,'tempory_share_token':temporary,'tempory_share_token_expire_at':int(time.time())+3600}.items():
 db.execute('insert into configs values(?,?)',(key,json.dumps(value)))
db.commit();db.close()
env={key:value for key,value in os.environ.items() if not key.startswith('KOMARI_')}
env.update(KOMARI_DB_TYPE='sqlite',KOMARI_DB_FILE=str(data/'komari.db'),KOMARI_LISTEN='127.0.0.1:'+str(port),KOMARI_CLOUDFLARED_TOKEN='')
log=(work/'server.log').open('w',encoding='utf-8')
process=subprocess.Popen([str(binary),'server'],cwd=work,env=env,stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0)
try:
 for _ in range(120):
  if process.poll() is not None:raise RuntimeError('fixture server exited; inspect private fixture log')
  try:
   if urllib.request.urlopen(base+'/ping',timeout=1).read()==b'pong':break
  except OSError:pass
  time.sleep(.25)
 else:raise RuntimeError('fixture startup timeout')
 creds=dict(line.split(': ',1) for line in (data/'initial-admin.txt').read_text(encoding='utf-8').strip().splitlines())
 jar=http.cookiejar.CookieJar();opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
 opener.open(base+entry).read()
 login=urllib.request.Request(base+'/api/login',data=json.dumps({'username':creds['Username'],'password':creds['Password']}).encode(),headers={'Content-Type':'application/json'})
 assert opener.open(login).status==200
 def rpc(method,params=None):
  req=urllib.request.Request(base+'/api/rpc2',data=json.dumps({'jsonrpc':'2.0','id':1,'method':method,'params':params or {}}).encode(),headers={'Content-Type':'application/json'})
  value=json.load(opener.open(req));assert 'error' not in value,method;return value['result']
 public=rpc('admin:addClient',{'name':'Visitor Node'})
 hidden=rpc('admin:addClient',{'name':'Hidden Operations Canary'})
 rpc('admin:editClient',{'uuid':hidden['uuid'],'hidden':True})
 rpc('admin:editSettings',{'theme':'default','theme_settings':{'guestDisplay':{'showPrice':True,'showExpiredAt':True}},'notification_enabled':False})
 # The onboarding suite covers tours separately. Seed the real theme settings
 # before starting this browser so a first-login tour cannot cover the editor.
 theme_settings={'guestDisplay':{'showPrice':True,'showExpiredAt':True},'_komari_onboarding_v1':{'seen':['install','workbench','notifications','markets'],'workbenchOpened':True}}
 theme_request=urllib.request.Request(base+'/api/admin/theme/settings?theme=default',data=json.dumps(theme_settings).encode(),headers={'Content-Type':'application/json'})
 assert opener.open(theme_request).status==200
 fixture={'base':base,'entry':entry,'view':view,'share':share,'temporary':temporary,'username':creds['Username'],'password':creds['Password'],'node':public['uuid'],'hidden':hidden['uuid'],'work':str(work)}
 info=work/'browser-fixture.json';info.write_text(json.dumps(fixture),encoding='utf-8')
 child_env={**os.environ,'PRIVATE_AUDIENCE_FIXTURE':str(info)}
 result=subprocess.run(['node',str(root/'script/private-audience.browser.mjs')],cwd=root,env=child_env,timeout=240)
 sys.exit(result.returncode)
finally:
 process.terminate()
 try:process.wait(timeout=15)
 except subprocess.TimeoutExpired:process.kill();process.wait(timeout=5)
 log.close()
