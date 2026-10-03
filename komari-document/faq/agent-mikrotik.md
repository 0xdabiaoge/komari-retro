# MikroTik 通过 HTTP 接入 Komari

RouterOS 无法运行常驻 Agent 或建立 WebSocket 时，可以使用定时脚本调用 Agent v2 JSON-RPC 接口。本文只支持 v2；旧版 `/api/clients/report`、`/api/clients/uploadBasicInfo` 和独立 Ping HTTP 接口已退役。

## 前提条件

- RouterOS 7.x，支持 `:serialize` 和 `:deserialize`
- 已在 Komari Server 中创建节点并获取 token
- Server 地址和 token 已填入下方脚本

所有请求发送到：

```text
POST {apiBase}/api/clients/v2/rpc?token={token}
```

请求体必须是 JSON-RPC 2.0 envelope。基础信息和实时报告可以直接使用以下脚本；事件拉取和 Ping/远程执行结果请参阅[通过 HTTP 接入 Komari](/faq/upload-via-http.html)。

## 基础信息脚本

```rsc
# Komari Agent v2 basic info for MikroTik RouterOS v7
:local apiBase "http://192.168.233.9:25774"
:local token   "replace-with-token"
:local rpcUrl  "$apiBase/api/clients/v2/rpc\?token=$token"

:local cpuName    [/system resource get cpu]
:local cpuCores   [/system resource get cpu-count]
:local arch       [/system resource get architecture-name]
:local rosVersion [/system resource get version]
:local memTotal   [/system resource get total-memory]
:local diskTotal 0
:do { :set diskTotal [/system resource get total-hdd-space] } on-error={}

:local info {
    "cpu_name"=$cpuName;
    "cpu_cores"=$cpuCores;
    "arch"=$arch;
    "os"=("RouterOS " . $rosVersion);
    "kernel_version"=$rosVersion;
    "mem_total"=$memTotal;
    "disk_total"=$diskTotal;
    "swap_total"=0;
    "gpu_name"="";
    "version"="routeros-agent/2.0";
    "virtualization"="None"
}
:local payload [:serialize to=json value={
    "jsonrpc"="2.0";
    "id"="basic-info";
    "method"="agent.basicInfo";
    "params"={"info"=$info}
}]

/tool fetch url=$rpcUrl mode=http http-method=post http-data=$payload \
    http-header-field="Content-Type:application/json" output=none
```

## 实时报告脚本

以下脚本展示最小报告字段。可按设备能力补充网络流量和连接数，并按 5-8 秒间隔执行。

```rsc
# Komari Agent v2 report for MikroTik RouterOS v7
:local apiBase "http://192.168.233.9:25774"
:local token   "replace-with-token"
:local rpcUrl  "$apiBase/api/clients/v2/rpc\?token=$token"

:local cpuLoad  [/system resource get cpu-load]
:local memTotal [/system resource get total-memory]
:local memFree  [/system resource get free-memory]
:local memUsed  ($memTotal - $memFree)
:local uptime   [/system resource get uptime]
:local uptimeSec [:tonum $uptime]

:local report {
    "cpu"={"usage"=$cpuLoad};
    "ram"={"total"=$memTotal;"used"=$memUsed};
    "swap"={"total"=0;"used"=0};
    "load"={"load1"=0;"load5"=0;"load15"=0};
    "disk"={"total"=0;"used"=0};
    "network"={"up"=0;"down"=0;"totalUp"=0;"totalDown"=0};
    "connections"={"tcp"=0;"udp"=0};
    "uptime"=$uptimeSec;
    "process"=0;
    "message"=""
}
:local payload [:serialize to=json value={
    "jsonrpc"="2.0";
    "id"=("report-" . [/system clock get time]);
    "method"="agent.report";
    "params"={"report"=$report;"ack_event_ids"={}}
}]

/tool fetch url=$rpcUrl mode=http http-method=post http-data=$payload \
    http-header-field="Content-Type:application/json" output=none
```
