#!/usr/bin/env python3
"""Scope origin and credential-log hardening to one existing HTTPS virtual host.

Run as root on the origin. Without --apply this only inventories and validates.
The configuration backup is retained; no containers, credentials, firewall or
SSH settings are modified. Cloudflare IPs are fetched over verified HTTPS.
"""
import argparse, datetime, hashlib, ipaddress, json, os, pathlib, re, shutil, subprocess, tempfile, urllib.request

def run(*args):
    p=subprocess.run(args,text=True,capture_output=True,timeout=45)
    if p.returncode: raise RuntimeError(p.stderr[-2000:])
    return p.stdout

def server_blocks(text):
    for match in re.finditer(r'(?m)^server\s*\{',text):
        depth=1;end=match.end()
        while depth and end<len(text):
            if text[end]=='{':depth+=1
            elif text[end]=='}':depth-=1
            end+=1
        if depth:raise ValueError("Unbalanced server block")
        yield match.start(),end,text[match.start():end]

def inventory():
    values=json.loads(run("docker","inspect",*run("docker","ps","-q").split()))
    return sorted([{"id":v["Id"],"name":v["Name"],"image":v["Image"],"pid":v["State"]["Pid"],"started":v["State"]["StartedAt"],"restarts":v["RestartCount"]} for v in values],key=lambda v:v["name"])

def enforce_existing(args,site,original,begin,end,block):
    if not args.apply:raise ValueError("Enforcement requires --apply after reviewing peer observations")
    prefix="komari_private_hardening"
    policy=pathlib.Path("/etc/nginx/conf.d/komari-private-hardening.conf").read_text()
    networks=[ipaddress.ip_network(n) for n in re.findall(r'^\s+([0-9a-f:./]+) 1;',policy,re.M)]
    if len(networks)<20:raise ValueError("Installed edge policy is incomplete")
    log=pathlib.Path("/var/log/nginx")/(args.host+".access.log")
    with log.open("rb") as stream:
        stream.seek(max(0,log.stat().st_size-2*1024*1024))
        lines=stream.read().decode("utf8","replace").splitlines()
    cloudflare=other=0
    for row in lines:
        if "/api/clients/[redacted]" not in row:continue
        match=re.search(r"peer=([^ ]+)",row)
        if not match:continue
        source=ipaddress.ip_address(match[1])
        if any(source in network for network in networks):cloudflare+=1
        else:other+=1
    if cloudflare<10 or other:raise ValueError("Need at least ten safe Agent observations and no untrusted origin peers")
    guard=f" if (${prefix}_edge = 0) {{ return 403; }}\n"
    if guard.strip() in block:raise ValueError("Origin restriction already installed")
    changed=block.replace("{","{\n"+guard,1)
    candidate=original[:begin]+changed+original[end:]
    # Split only this name out of a shared HTTP redirect virtual host. This
    # prevents HTTP capability URLs falling back to the global raw request log.
    http_blocks=[b for b in server_blocks(candidate) if re.search(r'\blisten\s+(?:\[::\]:)?80\b',b[2]) and args.host in re.search(r'\bserver_name\s+([^;]+);',b[2]).group(1).split()]
    if len(http_blocks)!=1:raise ValueError("Expected one matching HTTP redirect host")
    hb,he,http=http_blocks[0]
    names=re.search(r'\bserver_name\s+([^;]+);',http).group(1).split()
    dedicated=re.sub(r'\bserver_name\s+[^;]+;',f"server_name {args.host};",http)
    dedicated=dedicated.replace("{","{\n"+guard+f" access_log /var/log/nginx/{args.host}.access.log ${prefix}_log;\n error_log /dev/null crit;\n",1)
    if len(names)>1:
        remaining=re.sub(r'\bserver_name\s+[^;]+;', "server_name "+" ".join(n for n in names if n!=args.host)+";",http)
        candidate=candidate[:hb]+remaining+candidate[he:]+"\n"+dedicated+"\n"
    else:candidate=candidate[:hb]+dedicated+candidate[he:]
    with tempfile.TemporaryDirectory(prefix="komari-nginx-enforce-") as folder:
        folder=pathlib.Path(folder);verify=folder/"nginx.conf"
        verify.write_text("events {}\nhttp {\nmap $http_upgrade $connection_upgrade { default upgrade; '' close; }\n"+policy+"\n"+changed+"\n"+dedicated+"\n}\n")
        run("nginx","-t","-p",str(folder)+"/","-c",str(verify))
    before=inventory()
    backup=pathlib.Path("/root")/(".komari-private-origin-"+datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ"))
    backup.mkdir(mode=0o700);shutil.copy2(site,backup/"site-before.conf")
    archive=backup/"restricted-old-logs";archive.mkdir(mode=0o700);moved=[]
    try:
        site.write_text(candidate);run("nginx","-t")
        for source in pathlib.Path("/var/log/nginx").glob(args.host+".*.log*"):
            if source.is_symlink() or not source.is_file():raise ValueError("Unexpected log entry")
            destination=archive/source.name
            source.rename(destination);moved.append((source,destination))
        log.touch(mode=0o600);os.chmod(log,0o600)
        run("systemctl","reload","nginx")
    except BaseException:
        site.write_text(original)
        for source,destination in moved:
            if source.exists():source.rename(backup/("failed-new-"+source.name))
            destination.rename(source)
        run("nginx","-t");run("systemctl","reload","nginx")
        raise
    after=inventory()
    if before!=after:raise RuntimeError("Container preservation mismatch")
    print(json.dumps({"applied":True,"origin_enforced":True,"http_and_https_logs_redacted":True,"observed_trusted_agent_requests":cloudflare,"untrusted_agent_peers":other,"containers_unchanged":True,"backup":str(backup),"old_log_files_restricted":len(moved)},indent=2))

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument("--host",required=True)
    parser.add_argument("--site",required=True)
    parser.add_argument("--apply",action="store_true")
    parser.add_argument("--log-only",action="store_true",help="Install safe logs first, retaining origin access for transport verification")
    parser.add_argument("--enforce-existing",action="store_true",help="Enforce an already observed safe-log policy and restrict old logs")
    args=parser.parse_args()
    if not re.fullmatch(r"[a-z0-9.-]+",args.host):raise ValueError("Invalid hostname")
    os.umask(0o077)
    site=pathlib.Path(args.site).resolve(strict=True)
    if not str(site).startswith("/etc/nginx/"):raise ValueError("Site must be under /etc/nginx")
    original=site.read_text()
    matches=[b for b in server_blocks(original) if re.search(r"\blisten\s+(?:\[::\]:)?443\b",b[2]) and re.search(r"\bserver_name\s+"+re.escape(args.host)+r"\s*;",b[2])]
    if len(matches)!=1:raise ValueError("Expected exactly one dedicated HTTPS server")
    begin,end,block=matches[0]
    if args.enforce_existing:
        return enforce_existing(args,site,original,begin,end,block)
    if "komari_private_hardening" in block:raise ValueError("Already hardened; review existing configuration instead")
    networks=[]
    for family in (4,6):
        request=urllib.request.Request(f"https://www.cloudflare.com/ips-v{family}/",headers={"User-Agent":"KomariOriginHardening"})
        with urllib.request.urlopen(request,timeout=20) as response:lines=response.read(16384).decode().splitlines()
        parsed=[ipaddress.ip_network(line.strip(),strict=True) for line in lines if line.strip()]
        if not (5<=len(parsed)<=100) or any(n.version!=family or n.prefixlen<8 for n in parsed):raise ValueError("Unexpected Cloudflare IP list")
        networks.extend(parsed)
    prefix="komari_private_hardening"
    policy_path=pathlib.Path("/etc/nginx/conf.d/komari-private-hardening.conf")
    if policy_path.exists():raise ValueError("Policy file already exists")
    ranges="\n".join(f" {net} 1;" for net in networks)
    policy=f"""# Generated for {args.host}; update Cloudflare ranges when upstream changes.
geo $realip_remote_addr ${prefix}_edge {{
 default 0;
 127.0.0.1/32 1;
 ::1/128 1;
{ranges}
}}
map $uri ${prefix}_path {{
 default "/[redacted]";
 "/" "/";
 ~^/api/clients/ "/api/clients/[redacted]";
 ~^/api/ "/api/[redacted]";
 ~^/(?:share-assets|private-assets)/ "/assets/[redacted]";
}}
map $uri ${prefix}_limit_key {{
 default "";
 ~^/(?:viewer-api/|api/rpc2$|api/clients$|shared(?:/|$)|s/) $binary_remote_addr;
}}
limit_conn_zone ${prefix}_limit_key zone=${prefix}_per_ip:2m;
log_format ${prefix}_log '$remote_addr peer=$realip_remote_addr [$time_local] "$request_method ${prefix}_path $server_protocol" $status $body_bytes_sent upstream=$upstream_status rt=$request_time urt=$upstream_response_time';
"""
    trust="\n".join(f" set_real_ip_from {net};" for net in networks)
    guard=f"""
 # {prefix}: only Cloudflare or local diagnostics may reach this vhost.
 real_ip_header CF-Connecting-IP;
 real_ip_recursive on;
{trust}
 if (${prefix}_edge = 0) {{ return 403; }}
 limit_conn ${prefix}_per_ip 32;
 limit_conn_status 429;
"""
    changed=block.replace("{","{"+guard,1)
    if args.log_only:changed=changed.replace(f" if (${prefix}_edge = 0) {{ return 403; }}\n","")
    changed,n=re.subn(r"access_log\s+[^;]+;",f"access_log /var/log/nginx/{args.host}.access.log ${prefix}_log;",changed)
    if n!=1:raise ValueError("Expected one site access_log")
    # Nginx error messages can embed the original URI and query even when the
    # access format is safe. Preserve failures in structured access logs instead.
    changed,n=re.subn(r"error_log\s+[^;]+;","error_log /dev/null crit;",changed)
    if n!=1:raise ValueError("Expected one site error_log")
    candidate=original[:begin]+changed+original[end:]
    assert original[:begin]==candidate[:begin] and original[end:]==candidate[len(original[:begin])+len(changed):]
    with tempfile.TemporaryDirectory(prefix="komari-nginx-check-") as folder:
        folder=pathlib.Path(folder)
        verify=folder/"nginx.conf"
        verify.write_text("events {}\nhttp {\nmap $http_upgrade $connection_upgrade { default upgrade; '' close; }\n"+policy+"\n"+changed+"\n}\n")
        run("nginx","-t","-p",str(folder)+"/","-c",str(verify))
    result={"host":args.host,"cloudflare_ipv4_ranges":sum(n.version==4 for n in networks),"cloudflare_ipv6_ranges":sum(n.version==6 for n in networks),"isolated_nginx_validation":True,"applied":False,"other_virtual_hosts_unchanged":True}
    if args.apply:
        log=pathlib.Path("/var/log/nginx")/(args.host+".access.log")
        direct_agent=0
        if log.exists():
            with log.open("rb") as stream:
                stream.seek(max(0,log.stat().st_size-2*1024*1024))
                for row in stream.read().decode("utf-8","replace").splitlines():
                    if not re.search(r'"(?:GET|POST) /api/clients/',row):continue
                    try:source=ipaddress.ip_address(row.split()[0])
                    except ValueError:continue
                    if source.is_loopback:continue
                    if not any(source in network for network in networks):direct_agent+=1
        if direct_agent and not args.log_only:raise RuntimeError("Old access logs may contain restored visitor IPs. Use --log-only and inspect peer= transport addresses before enforcing origin restriction")
        result["legacy_agent_rows_with_non_cf_address"]=direct_agent
        before=inventory()
        stamp=datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        backup=pathlib.Path("/root")/(".komari-private-hardening-"+stamp);backup.mkdir(mode=0o700)
        shutil.copy2(site,backup/"site-before.conf")
        (backup/"manifest.json").write_text(json.dumps({"site":str(site),"new_policy":str(policy_path),"sha256_before":hashlib.sha256(original.encode()).hexdigest(),"containers":before},indent=2))
        try:
            policy_path.write_text(policy);os.chmod(policy_path,0o644)
            site.write_text(candidate)
            run("nginx","-t")
            run("systemctl","reload","nginx")
        except BaseException:
            site.write_text(original)
            if policy_path.exists():policy_path.unlink()
            run("nginx","-t");run("systemctl","reload","nginx")
            raise
        after=inventory()
        if before!=after:raise RuntimeError("Container preservation mismatch; inspect before any further action")
        units=run("systemctl","is-active","ssh","nginx","docker","fail2ban").splitlines()
        if units!=["active"]*4:raise RuntimeError("Unit health check failed")
        result.update(applied=True,backup=str(backup),containers_unchanged=True,units_active=True)
    print(json.dumps(result,indent=2))
if __name__=="__main__":main()
