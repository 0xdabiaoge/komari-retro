import os,subprocess,json,hashlib,shutil,time,sqlite3,urllib.request,signal
from pathlib import Path
BASE=Path(__file__).resolve().parents[1]
ROOT=BASE/'.codex/build/linux-installer-fixture';ROOT.mkdir(parents=True,exist_ok=True)
source=(BASE/'komari-agent/install.sh').read_text(encoding='utf-8')
prefix=source.split('# Set executable permissions')[0]
def check(port,name,fn):
    fn()
    print('PASS',name,flush=True)
payload=b'verified-candidate'
for scenario in ['download-failure','checksum-failure','success']:
    folder=ROOT/scenario;folder.mkdir(exist_ok=True);mock=folder/'bin';mock.mkdir(exist_ok=True);target=folder/'install';target.mkdir(exist_ok=True)
    (target/'agent').write_bytes(b'original-agent');events=folder/'events';events.write_text('')
    script=folder/'install-prefix.sh';script.write_text(prefix.replace('/etc/systemd/system/',str(folder/'fake-systemd')+'/'))
    curl=mock/'curl';curl.write_text('''#!/usr/bin/python3
import os,sys,hashlib
from pathlib import Path
args=sys.argv[1:];url=args[-1]
with open(os.environ['FIXTURE_EVENTS'],'a') as f:f.write(('checksum' if url.endswith('.sha256') else 'download')+chr(10))
if os.environ['FIXTURE_CASE']=='download-failure':sys.exit(22)
if url.endswith('.sha256'):
 print(('0'*64 if os.environ['FIXTURE_CASE']=='checksum-failure' else hashlib.sha256(b'verified-candidate').hexdigest())+'  agent')
else:Path(args[args.index('-o')+1]).write_bytes(b'verified-candidate')
''');curl.chmod(0o755)
    systemctl=mock/'systemctl';systemctl.write_text('''#!/bin/sh
if [ "$1" = --user ]; then shift; fi
if [ "$1" = list-unit-files ]; then echo "komari-acceptance-download-fixture.service enabled"; else echo "$1" >> "$FIXTURE_EVENTS"; fi
exit 0
''');systemctl.chmod(0o755)
    env=os.environ.copy();env.update(PATH=str(mock)+':'+env['PATH'],FIXTURE_CASE=scenario,FIXTURE_EVENTS=str(events),XDG_CONFIG_HOME=str(folder/'config'))
    p=subprocess.run(['bash',str(script),'--install-dir',str(target),'--install-service-name','komari-acceptance-download-fixture','--install-version','v1.2.13','--install-no-mirror','-e','http://127.0.0.1:1','-t','isolated-token'],capture_output=True,text=True,env=env,timeout=45)
    (folder/'run.log').write_text(p.stdout+p.stderr)
    def verify():
        actions=events.read_text().splitlines()
        if scenario=='success':
            assert p.returncode==0,(p.returncode,p.stderr)
            assert (target/'agent').read_bytes()==payload
            assert (target/'.agent.previous').read_bytes()==b'original-agent'
            assert actions.index('checksum')<actions.index('stop'),actions
        else:
            assert p.returncode!=0
            assert (target/'agent').read_bytes()==b'original-agent'
            assert 'stop' not in actions and 'disable' not in actions,actions
        assert not list(target.glob('.agent-download.*'))
        return {'scenario':scenario,'old_agent_preserved_on_failure':scenario!='success','actions':actions,'scope':'real installer download flow; mocked network and service control'}
    check(0,'linux-installer-'+scenario,verify)
print('INSTALLER_FIXTURES_DONE',flush=True)
