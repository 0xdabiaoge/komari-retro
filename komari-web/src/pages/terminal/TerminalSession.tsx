import { useEffect, useRef, useState } from "react";
import { Terminal, type ITerminalOptions } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { SearchAddon } from "@xterm/addon-search";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import { toast } from "sonner";
import {
  defaultPreferences,
  terminalThemes,
  type TerminalPreferences,
} from "./terminalPreferences";

export type TerminalStatus =
  "connecting" | "connected" | "reconnecting" | "disconnected" | "closed";
export interface TerminalSessionApi {
  terminal: Terminal;
  searchAddon: SearchAddon;
  send: (data: string | Uint8Array) => void;
  paste: (text: string) => void;
  reconnect: () => void;
  fit: () => void;
  exportText: () => string;
}
interface Props {
  uuid: string;
  active: boolean;
  fontSize?: number;
  padding?: number;
  preferences?: TerminalPreferences;
  disconnectMessage: string;
  onApiChange: (api: TerminalSessionApi | null) => void;
  onStatusChange?: (status: TerminalStatus) => void;
}
export default function TerminalSession({
  uuid,
  active,
  fontSize,
  padding,
  preferences = defaultPreferences,
  onApiChange,
  onStatusChange,
}: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const apiRef = useRef<TerminalSessionApi | null>(null);
  const callbacks = useRef({ onApiChange, onStatusChange });
  const optionsRef = useRef({ preferences, fontSize });
  const activeRef = useRef(active);
  const [pendingPaste, setPendingPaste] = useState<string | null>(null);
  callbacks.current = { onApiChange, onStatusChange };
  optionsRef.current = { preferences, fontSize };
  activeRef.current = active;

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const initial = optionsRef.current;
    const term = new Terminal({
      ...initial.preferences.terminalOptions,
      fontSize:
        initial.fontSize ?? initial.preferences.terminalOptions.fontSize,
      theme: terminalThemes[initial.preferences.themeName],
      allowProposedApi: true,
      overviewRuler: { width: 4 },
      cursorStyle: "bar",
    });
    const fitAddon = new FitAddon(),
      searchAddon = new SearchAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(searchAddon);
    term.loadAddon(
      new WebLinksAddon((_event, uri) => {
        if (/^https?:\/\//i.test(uri))
          window.open(uri, "_blank", "noopener,noreferrer");
      }),
    );
    term.loadAddon(new Unicode11Addon());
    term.unicode.activeVersion = "11";
    term.open(host);
    let disposed = false,
      ended = false,
      socket: WebSocket | null = null;
    let requestID: string | null = null,
      queuedSeq = 0,
      renderedSeq = 0;
    let offeredSeq = 0, epoch = 0;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let heartbeat: ReturnType<typeof setInterval> | undefined;
    let deadline = 0,
      attempts = 0,
      fitFrame = 0;
    const status = (value: TerminalStatus) => {
      if (!disposed) callbacks.current.onStatusChange?.(value);
    };
    const control = (value: object) => {
      if (socket?.readyState === WebSocket.OPEN)
        socket.send(JSON.stringify(value));
    };
    const send = (data: string | Uint8Array) => {
      if (!disposed && !ended && socket?.readyState === WebSocket.OPEN) {
        if (socket.bufferedAmount > 128 * 1024) {
          toast.error("输入发送积压，请等待连接恢复");
          return;
        }
        socket.send(
          typeof data === "string" ? new TextEncoder().encode(data) : data,
        );
      }
    };
    const fit = () => {
      if (disposed || !host.clientWidth || !host.clientHeight) return;
      cancelAnimationFrame(fitFrame);
      fitFrame = requestAnimationFrame(() => {
        if (disposed) return;
        fitAddon.fit();
        control({ type: "resize", cols: term.cols, rows: term.rows });
      });
    };
    const paste = (text: string) => {
      if (text.length > 64 * 1024) {
        toast.error("单次粘贴不能超过 64 KB，请使用文件上传");
        return;
      }
      if (/[\r\n]/.test(text)) {
        setPendingPaste(text);
        return;
      }
      term.paste(text);
      term.focus();
    };
    const copy = async () => {
      try {
        await navigator.clipboard.writeText(term.getSelection());
      } catch {
        toast.error("复制失败，请检查浏览器剪贴板权限");
      }
    };
    const pasteClipboard = async () => {
      try {
        paste(await navigator.clipboard.readText());
      } catch {
        toast.error("读取剪贴板失败，请使用系统粘贴快捷键");
      }
    };
    const stopTimers = () => {
      clearTimeout(retryTimer);
      clearInterval(heartbeat);
    };
    const connect = () => {
      if (disposed) return;
      offeredSeq = 0;
      status(attempts ? "reconnecting" : "connecting");
      const query = new URLSearchParams({ flow: "1" });
      if (requestID) query.set("request_id", requestID);
      const ws = new WebSocket(
        `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/api/admin/client/${encodeURIComponent(uuid)}/terminal?${query}`,
      );
      socket = ws;
      ws.binaryType = "arraybuffer";
      let stableTimer: ReturnType<typeof setTimeout> | undefined;
      ws.onopen = () => {
        if (socket !== ws || disposed) return;
        stableTimer = setTimeout(() => {
          attempts = 0;
          deadline = 0;
        }, 10000);
        heartbeat = setInterval(() => control({ type: "heartbeat" }), 5000);
        fit();
      };
      ws.onmessage = (event) => {
        if (disposed || socket !== ws) return;
        if (typeof event.data === "string") {
          let message: {
            type?: string;
            request_id?: string;
            ready?: boolean;
            seq?: number;
            data?: string;
            message?: string;
          };
          try {
            message = JSON.parse(event.data);
          } catch {
            term.write(event.data);
            return;
          }
          if (message.type === "session" && message.request_id) {
            requestID = message.request_id;
            if (message.ready) status("connected");
            return;
          }
          if (message.type === "connected") {
            status("connected");
            fit();
            return;
          }
          if (message.type === "closed") {
            ended = true;
            stopTimers();
            status("closed");
            term.writeln("\r\n[终端已结束；可点击新建连接]");
            return;
          }
          if (message.type === "reconnecting") {
            status("reconnecting");
            if (message.message) term.writeln(`\r\n[${message.message}]`);
            return;
          }
          if (
            message.type === "output" &&
            Number.isSafeInteger(message.seq) &&
            typeof message.data === "string"
          ) {
            const seq = message.seq!;
            offeredSeq = Math.max(offeredSeq, seq);
            if (seq <= queuedSeq) {
              control({ type: "ack", seq: Math.min(renderedSeq, offeredSeq) });
              return;
            }
            if (seq !== queuedSeq + 1 || message.data.length > 32768) {
              ended = true;
              status("closed");
              control({ type: "close" });
              ws.close();
              toast.error("终端输出顺序异常，已停止连接");
              return;
            }
            const bytes = Uint8Array.from(atob(message.data), (c) =>
              c.charCodeAt(0),
            );
            queuedSeq = seq;
            const writeEpoch = epoch;
            term.write(bytes, () => {
              if (!disposed && writeEpoch === epoch) {
                renderedSeq = seq;
                control({ type: "ack", seq: Math.min(seq, offeredSeq) });
              }
            });
            return;
          }
          return;
        }
        // Compatibility with an older server while a release is being rolled out.
        if (event.data instanceof ArrayBuffer)
          term.write(new Uint8Array(event.data));
      };
      ws.onclose = (event) => {
        clearTimeout(stableTimer);
        clearInterval(heartbeat);
        if (socket !== ws || disposed || ended) return;
        socket = null;
        if (event.code === 1000 || event.code === 4001 || event.code === 4003) {
          ended = true;
          status("closed");
          return;
        }
        if (!deadline) deadline = Date.now() + 5 * 60 * 1000;
        if (Date.now() >= deadline) {
          ended = true;
          status("closed");
          term.writeln("\r\n[恢复窗口已结束；请检查节点和登录状态，再点击新建连接]");
          return;
        }
        status("reconnecting");
        retryTimer = setTimeout(
          connect,
          Math.min(1000 * 2 ** Math.min(attempts++, 4), 15000),
        );
      };
      ws.onerror = () => {
        /* onclose handles bounded retry without creating a replacement shell. */
      };
    };
    const reconnect = () => {
      stopTimers();
      const previous = socket;
      socket = null;
      previous?.close();
      if (ended) {
        epoch++;
        requestID = null;
        queuedSeq = 0;
        renderedSeq = 0;
        term.reset();
      }
      ended = false;
      attempts = 0;
      deadline = 0;
      connect();
    };
    const onPaste = (event: ClipboardEvent) => {
      if (!activeRef.current) return;
      const text = event.clipboardData?.getData("text/plain");
      if (text !== undefined) {
        event.preventDefault();
        event.stopImmediatePropagation();
        paste(text);
      }
    };
    host.addEventListener("paste", onPaste, true);
    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown") return true;
      if (
        (event.ctrlKey || event.metaKey) &&
        event.key.toLowerCase() === "c" &&
        term.hasSelection()
      ) {
        event.preventDefault();
        void copy();
        return false;
      }
      if (event.ctrlKey && event.shiftKey && event.key.toLowerCase() === "v") {
        event.preventDefault();
        void pasteClipboard();
        return false;
      }
      if (
        (event.ctrlKey && (event.altKey || event.shiftKey)) ||
        (event.altKey && event.key === "Enter")
      )
        return false;
      return true;
    });
    const contextMenu = (event: MouseEvent) => {
      event.preventDefault();
      if (term.hasSelection()) void copy();
      else void pasteClipboard();
    };
    host.addEventListener("contextmenu", contextMenu);
    const dataSub = term.onData(send);
    const binarySub = term.onBinary((value) =>
      send(Uint8Array.from(value, (c) => c.charCodeAt(0) & 255)),
    );
    const observer = new ResizeObserver(fit);
    observer.observe(host);
    window.visualViewport?.addEventListener("resize", fit);
    const api: TerminalSessionApi = {
      terminal: term,
      searchAddon,
      send,
      paste,
      reconnect,
      fit,
      exportText: () => {
        const buffer = term.buffer.active,
          lines: string[] = [];
        for (let i = 0; i < buffer.length; i++) {
          const line = buffer.getLine(i);
          if (line) lines.push(line.translateToString(true));
        }
        return lines.join("\n").trimEnd();
      },
    };
    apiRef.current = api;
    callbacks.current.onApiChange(api);
    connect();
    fit();
    if (activeRef.current) term.focus();
    return () => {
      disposed = true;
      stopTimers();
      cancelAnimationFrame(fitFrame);
      observer.disconnect();
      window.visualViewport?.removeEventListener("resize", fit);
      host.removeEventListener("paste", onPaste, true);
      host.removeEventListener("contextmenu", contextMenu);
      if (socket?.readyState === WebSocket.OPEN) control({ type: "close" });
      socket?.close();
      socket = null;
      dataSub.dispose();
      binarySub.dispose();
      term.dispose();
      apiRef.current = null;
      callbacks.current.onApiChange(null);
    };
  }, [uuid]);

  useEffect(() => {
    const api = apiRef.current;
    if (!api) return;
    const options: Partial<ITerminalOptions> = {
      ...preferences.terminalOptions,
      fontSize: fontSize ?? preferences.terminalOptions.fontSize,
      theme: terminalThemes[preferences.themeName],
    };
    Object.assign(api.terminal.options, options);
    api.fit();
  }, [preferences, fontSize, padding]);
  useEffect(() => {
    if (active) {
      apiRef.current?.fit();
      apiRef.current?.terminal.focus();
    }
  }, [active]);

  return (
    <div
      className={`km-terminal-session terminal-page absolute inset-0 h-full w-full overflow-hidden ${active ? "is-active z-[1]" : "invisible pointer-events-none"}`}
      style={{
        background: terminalThemes[preferences.themeName].background,
        padding: padding ?? preferences.terminalPadding,
      }}
      aria-hidden={!active}
    >
      <div
        ref={hostRef}
        className="km-terminal-xterm-viewport h-full min-h-0 w-full overflow-hidden"
      />
      {pendingPaste !== null && (
        <div className="km-terminal-paste-overlay absolute inset-0 z-20 flex items-center justify-center bg-black/70 p-4">
          <div
            role="dialog"
            aria-modal="true"
            aria-label="确认多行粘贴"
            className="w-full max-w-lg rounded-xl border border-slate-600 bg-slate-900 p-5 text-slate-100 shadow-2xl"
          >
            <h2 className="font-semibold">确认多行粘贴</h2>
            <p className="my-2 text-sm text-slate-300">
              内容包含换行，可能执行多条命令。请先检查：
            </p>
            <pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded bg-black p-3 text-xs">
              {pendingPaste}
            </pre>
            <div className="mt-4 flex justify-end gap-3">
              <button autoFocus onClick={() => setPendingPaste(null)}>
                取消
              </button>
              <button
                className="rounded bg-cyan-700 px-3 py-2"
                onClick={() => {
                  apiRef.current?.terminal.paste(pendingPaste);
                  setPendingPaste(null);
                  apiRef.current?.terminal.focus();
                }}
              >
                确认粘贴
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
