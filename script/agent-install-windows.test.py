from pathlib import Path
import tempfile,subprocess,json,os
base=Path(__file__).resolve().parents[1]
source=(base/'komari-agent/install.ps1').read_text(encoding='utf-8')
flow=source[source.index('# Construct download URL'):source.index('# Register and start service')]
results=[]
root=base/'.codex/build/windows-installer-fixture';root.mkdir(parents=True,exist_ok=True)
for scenario in ['download-failure','checksum-failure','success']:
    folder=root/scenario;folder.mkdir(exist_ok=True);(folder/'komari-agent.exe').write_bytes(b'original-agent')
    test=r'''
$ErrorActionPreference='Stop'
$InstallDir=$PSScriptRoot
$AgentPath=Join-Path $InstallDir 'komari-agent.exe'
$arch='amd64';$versionToInstall='v1.2.13';$GitHubProxy=''
$Scenario='SCENARIO'
$Events=Join-Path $InstallDir 'events'
Set-Content -LiteralPath $Events -Value ''
function Log-Info { param($Message) }
function Log-Success { param($Message) }
function Log-Error { param($Message) Write-Host $Message }
function Uninstall-Previous {
 Add-Content -LiteralPath $Events -Value 'stop'
 Remove-Item -LiteralPath $AgentPath -Force
}
function Invoke-WebRequest {
 param($Uri,$OutFile,[switch]$UseBasicParsing,$ErrorAction)
 if ($Scenario -eq 'download-failure') { throw 'isolated network error' }
 if ($Uri.EndsWith('.sha256')) {
  Add-Content -LiteralPath $Events -Value 'checksum'
  $Hash=if ($Scenario -eq 'checksum-failure') { '0'*64 } else { (Get-FileHash -LiteralPath $OutFileForHash -Algorithm SHA256).Hash }
  return @{Content=[Text.Encoding]::UTF8.GetBytes($Hash+' agent.exe')}
 }
 Add-Content -LiteralPath $Events -Value 'download'
 [IO.File]::WriteAllBytes($OutFile,[Text.Encoding]::UTF8.GetBytes('verified-candidate'))
 $script:OutFileForHash=$OutFile
}
'''.replace('SCENARIO',scenario)+flow
    script=folder/'case.ps1';script.write_text(test,encoding='utf-8-sig')
    env=os.environ.copy();env['PSModulePath']=str(Path(os.environ['SystemRoot'])/'System32/WindowsPowerShell/v1.0/Modules')
    p=subprocess.run(['powershell.exe','-NoProfile','-ExecutionPolicy','Bypass','-File',str(script)],capture_output=True,text=True,env=env)
    events=(folder/'events').read_text(encoding='utf-8-sig').split()
    try:
        if scenario=='success':
            assert p.returncode==0,p.stderr
            assert (folder/'komari-agent.exe').read_bytes()==b'verified-candidate'
            assert (folder/'.agent.previous.exe').read_bytes()==b'original-agent'
            assert events.index('checksum')<events.index('stop')
        else:
            assert p.returncode!=0
            assert (folder/'komari-agent.exe').read_bytes()==b'original-agent'
            assert 'stop' not in events
        assert not list(folder.glob('.agent-download-*'))
        results.append({'case':scenario,'status':'PASS','evidence':events})
    except Exception as error:results.append({'case':scenario,'status':'FAIL','error':str(error),'stderr':p.stderr,'stdout':p.stdout})

# Exercise the real native-process boundary, including nested AppParameters
# quotes. A PowerShell function mock would hide the Windows argument loss.
quote_function=source[source.index('function ConvertTo-CommandLineArg {'):source.index('# Ensure running as Administrator.')]
nssm_function=source[source.index('function Invoke-NssmChecked {'):source.index("try {\n    Invoke-NssmChecked @('install'")]
with tempfile.TemporaryDirectory(prefix='nssm arguments ') as directory:
    folder=Path(directory)
    script=folder/'arguments.ps1'
    script.write_text(r'''
$ErrorActionPreference='Stop'
$fakeExe=Join-Path $PSScriptRoot 'native args.exe'
Add-Type -OutputAssembly $fakeExe -OutputType ConsoleApplication -TypeDefinition @'
using System;
using System.IO;
using System.Text;
public class NativeArgs {
 public static int Main(string[] args) {
  if (args[0] == "fail") return 3;
  File.WriteAllLines(Environment.GetEnvironmentVariable("KOMARI_ARGS_RESULT"),
   Array.ConvertAll(args, a => Convert.ToBase64String(Encoding.UTF8.GetBytes(a))));
  return 0;
 }
}
'@
$env:KOMARI_ARGS_RESULT=Join-Path $PSScriptRoot 'received.txt'
$nssmCmd=@{Source=$fakeExe}
'''+quote_function+nssm_function+r'''
$app='C:\agent directory\komari-agent.exe'
$config='C:\config directory\config.json'
$parameters='--config '+(ConvertTo-CommandLineArg $config)
$expected=@('install','fixture service',$app,$parameters)
Invoke-NssmChecked $expected
$received=@(Get-Content -LiteralPath $env:KOMARI_ARGS_RESULT | ForEach-Object {
 [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($_))
})
if ($received.Count -ne $expected.Count) { throw 'Native argument count changed' }
for ($i=0;$i -lt $expected.Count;$i++) {
 if ($received[$i] -cne $expected[$i]) { throw "Native argument $i changed" }
}
$rejected=$false
try { Invoke-NssmChecked @('fail') } catch { $rejected=$true }
if (-not $rejected) { throw 'Native process failure was ignored' }
''',encoding='utf-8-sig')
    p=subprocess.run(['powershell.exe','-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',str(script)],capture_output=True,text=True,env=env)
    results.append({'case':'native-service-argument-quoting-and-exit-status','status':'PASS' if p.returncode==0 else 'FAIL','evidence':{'exit_code':p.returncode,'stderr':p.stderr}})
(root/'results.json').write_text(json.dumps(results,indent=2),encoding='utf-8')
print(json.dumps(results,indent=2))
assert all(r['status']=='PASS' for r in results)


