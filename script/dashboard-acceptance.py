"""Real dashboard browser acceptance against a disposable loopback server."""
import json, os, pathlib, secrets, shutil, socket, sqlite3, subprocess, sys, tempfile, time, urllib.request

root=pathlib.Path(__file__).resolve().parents[1]
binary=pathlib.Path(sys.argv[1]).resolve()
parent=root/'.codex/dashboard-fixtures';parent.mkdir(parents=True,exist_ok=True)
work=pathlib.Path(tempfile.mkdtemp(prefix='fixture-',dir=parent)).resolve()
data=work/'data';data.mkdir()
with socket.socket() as probe:
 probe.bind(('127.0.0.1',0));port=probe.getsockname()[1]
base='http://127.0.0.1:'+str(port)
entry='/entry-'+secrets.token_hex(20)
db=sqlite3.connect(data/'komari.db');db.execute('create table configs(key text primary key,value text)')
for key,value in {'eula_accepted':True,'notification_enabled':False,'login_notification':False,'geo_ip_enabled':False,'private_site':True,'admin_path':entry,'theme':'default','theme_settings':{'_komari_onboarding_v1':{'completed':True,'skipped':True,'completedAt':int(time.time()*1000),'seenSteps':[]}}}.items():
 db.execute('insert into configs values(?,?)',(key,json.dumps(value)))
db.commit();db.close()
env={k:v for k,v in os.environ.items() if not k.startswith('KOMARI_')}
env.update(KOMARI_DB_TYPE='sqlite',KOMARI_DB_FILE=str(data/'komari.db'),KOMARI_LISTEN='127.0.0.1:'+str(port),KOMARI_CLOUDFLARED_TOKEN='')
log=(work/'server.log').open('w',encoding='utf-8')
p=subprocess.Popen([str(binary),'server'],cwd=work,env=env,stdout=log,stderr=subprocess.STDOUT,creationflags=subprocess.CREATE_NO_WINDOW if os.name=='nt' else 0)
try:
 for _ in range(120):
  if p.poll() is not None:raise RuntimeError('isolated dashboard server exited')
  try:
   if urllib.request.urlopen(base+'/ping',timeout=1).read()==b'pong':break
  except OSError:pass
  time.sleep(.25)
 else:raise RuntimeError('isolated dashboard startup timed out')
 creds=dict(line.split(': ',1) for line in (data/'initial-admin.txt').read_text(encoding='utf-8').strip().splitlines())
 d=sqlite3.connect(data/'komari.db',timeout=10)
 for sql in [
  "WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<500) INSERT INTO clients(uuid,token,name,hidden,mem_total) SELECT printf('%036d',i),printf('fixture-token-%d',i),printf('fixture-node-%d',i),i%10=0,1024 FROM n",
  "WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<36) INSERT INTO ping_tasks(id,name,type,target,interval,clients,all_clients,weight) SELECT i,printf('fixture-task-%d',i),'icmp','192.0.2.1',60,'[]',0,i FROM n",
  "WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<3) INSERT INTO records(client,time,cpu,ram,ram_total,net_in,net_out,traffic_up,traffic_down) SELECT c.uuid,datetime('now',printf('-%d minutes',n.i)),25,512,1024,200,100,60,120 FROM clients c CROSS JOIN n",
  "WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<3) INSERT INTO ping_records(client,task_id,time,value) SELECT c.uuid,t.id,datetime('now',printf('-%d minutes',n.i)),CASE WHEN t.id=36 THEN -1 ELSE n.i*10 END FROM clients c CROSS JOIN ping_tasks t CROSS JOIN n",
 ]:d.execute(sql)
 d.commit();d.close()
 fixture={'base':base,'entry':entry,'username':creds['Username'],'password':creds['Password']}
 go=os.environ.get('KOMARI_TEST_GO','go')
 probe=subprocess.run([go,'run','./script/agent-scale'],cwd=root,input=json.dumps({'base':base}),text=True,env=env,timeout=90)
 if probe.returncode:
  destination=root/'.codex/audit';destination.mkdir(parents=True,exist_ok=True)
  shutil.copyfile(work/'server.log',destination/'dashboard-browser-server-failure.log')
 assert probe.returncode==0,'500-Agent WebSocket acceptance failed'
 result=subprocess.run(['node',str(root/'komari-web/script/dashboard.browser.mjs')],cwd=root,input=json.dumps(fixture),text=True,env=env,timeout=180)
 if result.returncode:
  destination=root/'.codex/audit';destination.mkdir(parents=True,exist_ok=True)
  shutil.copyfile(work/'server.log',destination/'dashboard-browser-server-failure.log')
 assert result.returncode==0,'dashboard browser acceptance failed'
finally:
 p.terminate()
 try:p.wait(timeout=15)
 except subprocess.TimeoutExpired:p.kill();p.wait()
 log.close()
 assert work.parent==parent.resolve() and not work.is_symlink()
 shutil.rmtree(work)
