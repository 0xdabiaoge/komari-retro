import pathlib,os,json,sqlite3,subprocess,secrets,socket,time,urllib.request,http.cookiejar,tempfile,shutil,sys
root=pathlib.Path(__file__).resolve().parents[1]
parent=root/'.codex/latency-history';parent.mkdir(parents=True,exist_ok=True);work=pathlib.Path(tempfile.mkdtemp(prefix='browser-fixture-',dir=parent)).resolve();data=work/'data';data.mkdir()
with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
base='http://127.0.0.1:'+str(port);entry='/entry-'+secrets.token_hex(24);view='/view-'+secrets.token_hex(24)
with sqlite3.connect(data/'komari.db') as d:
 d.execute('CREATE TABLE configs(key text primary key,value text)')
 for k,v in {'eula_accepted':True,'notification_enabled':False,'login_notification':False,'geo_ip_enabled':False,'private_site':True,'admin_path':entry,'admin_view_path':view,'theme':'retro','record_preserve_time':24,'ping_record_preserve_time':24}.items():d.execute('INSERT INTO configs VALUES (?,?)',(k,json.dumps(v)))
d.close()
env={k:v for k,v in os.environ.items() if not k.startswith('KOMARI_')};env.update(KOMARI_DB_TYPE='sqlite',KOMARI_DB_FILE=str(data/'komari.db'),KOMARI_LISTEN='127.0.0.1:'+str(port),KOMARI_CLOUDFLARED_TOKEN='',TZ='UTC')
log=(work/'server.log').open('w',encoding='utf-8');p=subprocess.Popen([str(pathlib.Path(sys.argv[1]).resolve()),'server'],cwd=work,env=env,stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0)
try:
 for _ in range(100):
  try:
   if urllib.request.urlopen(base+'/ping',timeout=1).read()==b'pong':break
  except OSError:pass
  time.sleep(.2)
 else:raise RuntimeError('isolated startup failed')
 creds=dict(x.split(': ',1) for x in (data/'initial-admin.txt').read_text().strip().splitlines())
 jar=http.cookiejar.CookieJar();o=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar));o.open(base+entry).read()
 o.open(urllib.request.Request(base+'/api/login',headers={'Content-Type':'application/json'},data=json.dumps({'username':creds['Username'],'password':creds['Password']}).encode())).read()
 def rpc(m,a={}):
  j=json.load(o.open(urllib.request.Request(base+'/api/rpc2',headers={'Content-Type':'application/json'},data=json.dumps({'jsonrpc':'2.0','id':1,'method':m,'params':a}).encode())));assert 'error' not in j,(m,j.get('error'));return j['result']
 node=rpc('admin:addClient',{'name':'Full Day Latency Node'});uid=node['uuid']
 with sqlite3.connect(data/'komari.db') as d:
  for i in range(36):d.execute('INSERT INTO ping_tasks(id,name,type,target,interval,clients,all_clients,weight) VALUES (?,?,?,?,?,?,?,?)',(970000+i,'Latency-'+str(i),'icmp','192.0.2.1',60,json.dumps([uid]),0,i))
  d.execute("WITH RECURSIVE n(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM n WHERE i<1439) INSERT INTO ping_records(client,task_id,time,value) SELECT ?,t.id,strftime('%Y-%m-%d %H:%M:%f','now','-86370 seconds',printf('+%d seconds',i*60)),CASE i%4 WHEN 0 THEN 0 WHEN 1 THEN 20 WHEN 2 THEN -1 ELSE 40 END FROM n CROSS JOIN ping_tasks t",(uid,))
 d.close()
 shares={}
 for name,hours in [('day',24),('hour',1),('live',0)]:shares[name]=rpc('admin:createViewerShare',{'name':'isolated-'+name,'nodes':[uid],'all_nodes':False,'history_hours':hours,'max_connections':8})['path']
 fixture={'base':base,'entry':entry,'view':view,'username':creds['Username'],'password':creds['Password'],'node':uid,'shares':shares}
 f=work/'fixture.json';f.write_text(json.dumps(fixture),encoding='utf-8')
 result=subprocess.run(['node',str(root/'script/latency-history.browser.mjs'),str(f)],cwd=root,timeout=240,env=env)
 assert result.returncode==0,'browser regression failed'
finally:
 p.terminate()
 try:p.wait(timeout=15)
 except subprocess.TimeoutExpired:p.kill();p.wait()
 log.close()
 shutil.copyfile(work/'server.log',parent/'latency-browser-server.log')
 assert work.parent==parent.resolve() and not work.is_symlink();shutil.rmtree(work)
