import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  BookOpen,
  Copy,
  Files,
  Search,
  Settings,
  Trash2,
  X,
} from "lucide-react";
import type { TerminalSessionApi, TerminalStatus } from "./TerminalSession";
import type {
  SavedCommand,
  TerminalPreferences,
  TerminalThemeName,
} from "./terminalPreferences";
import "./workbench.css";

const labels: Record<TerminalStatus, string> = {
  connecting: "连接中",
  connected: "已连接",
  reconnecting: "正在恢复",
  disconnected: "连接已断开",
  closed: "会话已结束",
};
interface Props {
  name: string;
  status: TerminalStatus;
  api: TerminalSessionApi | null;
  preferences: TerminalPreferences;
  saving: boolean;
  save: (value: TerminalPreferences) => Promise<boolean>;
  onSearch: () => void;
  onFiles: () => void;
}
export default function WorkbenchTools({
  name,
  status,
  api,
  preferences,
  saving,
  save,
  onSearch,
  onFiles,
}: Props) {
  const [panel, setPanel] = useState<"settings" | "commands" | null>(null);
  const [draft, setDraft] = useState(preferences);
  const [command, setCommand] = useState<SavedCommand>({
    id: "",
    name: "",
    command: "",
    description: "",
  });
  const [selected, setSelected] = useState<SavedCommand | null>(null);
  const panelRef = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!panel) return;
    const key = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const items = [
        ...(panelRef.current?.querySelectorAll<HTMLElement>(
          "button:not(:disabled), input, select, textarea",
        ) ?? []),
      ];
      const first = items[0],
        last = items.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", key);
    return () => document.removeEventListener("keydown", key);
  }, [panel]);
  const open = (next: "settings" | "commands") => {
    setDraft(preferences);
    setPanel(panel === next ? null : next);
  };
  const patchOption = (key: string, value: string | number | boolean) =>
    setDraft((v) => ({
      ...v,
      terminalOptions: { ...v.terminalOptions, [key]: value },
    }));
  const saveCommand = async () => {
    const item = {
      ...command,
      id: command.id || crypto.randomUUID(),
      name: command.name.trim(),
    };
    const list = preferences.savedCommands.filter((c) => c.id !== item.id);
    if (await save({ ...preferences, savedCommands: [...list, item] })) {
      setCommand({ id: "", name: "", command: "", description: "" });
      setSelected(null);
    }
  };
  return (
    <>
      <header className="km-workbench-brand">
        <a href="/admin" aria-label="返回后台">
          <ArrowLeft size={16} />
        </a>
        <strong>
          Komari <span>Retro</span>
        </strong>
        <span className="km-workbench-caption">远程工作台</span>
        <span className="km-workbench-server">
          {name || "选择节点开始连接"}
        </span>
      </header>
      <div className="km-workbench-tools">
        <span className={`km-connection-status ${status}`} role="status">
          <i />
          {api ? labels[status] : "未选择节点"}
        </span>
        <button disabled={!api} onClick={() => api?.reconnect()}>
          {status === "closed" ? "新建连接" : "重新连接"}
        </button>
        <span className="km-tools-spacer" />
        <button disabled={!api} onClick={onSearch}>
          <Search size={14} />
          搜索
        </button>
        <button
          disabled={!api}
          onClick={() => {
            const text = api?.terminal.getSelection();
            if (text) void navigator.clipboard.writeText(text).catch(() => {});
          }}
        >
          <Copy size={14} />
          复制
        </button>
        <button disabled={!api} onClick={() => api?.terminal.clear()}>
          <Trash2 size={14} />
          清屏
        </button>
        <button
          onClick={() => open("commands")}
          aria-expanded={panel === "commands"}
        >
          <BookOpen size={14} />
          常用命令
        </button>
        <button disabled={!api} onClick={onFiles}>
          <Files size={14} />
          文件
        </button>
        <button
          onClick={() => open("settings")}
          aria-expanded={panel === "settings"}
        >
          <Settings size={14} />
          设置
        </button>
      </div>
      {panel && (
        <div
          className="km-workbench-panel-overlay"
          onMouseDown={(e) => {
            if (e.target === e.currentTarget) setPanel(null);
          }}
        >
          <section
            ref={panelRef}
            className="km-workbench-panel"
            role="dialog"
            aria-modal="true"
            aria-label={panel === "settings" ? "工作台设置" : "常用命令"}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.stopPropagation();
                setPanel(null);
              }
            }}
          >
            <div className="km-workbench-panel-title">
              <h2>{panel === "settings" ? "工作台设置" : "常用命令"}</h2>
              <button
                autoFocus
                onClick={() => setPanel(null)}
                aria-label="关闭面板"
              >
                <X size={18} />
              </button>
            </div>
            {panel === "settings" ? (
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void save(draft).then((ok) => {
                    if (ok) setPanel(null);
                  });
                }}
              >
                <p>保存后应用到所有打开的终端，不会重建 Shell。</p>
                <label>
                  配色
                  <select
                    aria-label="配色"
                    value={draft.themeName}
                    onChange={(e) =>
                      setDraft((v) => ({
                        ...v,
                        themeName: e.target.value as TerminalThemeName,
                      }))
                    }
                  >
                    <option value="dark">深色</option>
                    <option value="light">浅色</option>
                    <option value="contrast">高对比度</option>
                  </select>
                </label>
                <label>
                  字号
                  <select
                    aria-label="字号"
                    value={draft.terminalOptions.fontSize}
                    onChange={(e) =>
                      patchOption("fontSize", Number(e.target.value))
                    }
                  >
                    {Array.from({ length: 23 }, (_, i) => i + 10).map((n) => (
                      <option key={n} value={n}>
                        {n} px
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  字体
                  <select
                    aria-label="字体"
                    value={draft.terminalOptions.fontFamily}
                    onChange={(e) => patchOption("fontFamily", e.target.value)}
                  >
                    <option value={preferences.terminalOptions.fontFamily}>
                      {preferences.terminalOptions.fontFamily}
                    </option>
                    <option value="Consolas, 'Noto Sans Mono CJK SC', monospace">
                      Consolas / 中文等宽回退
                    </option>
                    <option value="'Cascadia Mono', monospace">
                      Cascadia Mono
                    </option>
                    <option value="monospace">系统等宽字体</option>
                  </select>
                </label>
                <label>
                  回滚行数
                  <select
                    aria-label="回滚行数"
                    value={draft.terminalOptions.scrollback}
                    onChange={(e) =>
                      patchOption("scrollback", Number(e.target.value))
                    }
                  >
                    {[1000, 5000, 10000].map((n) => (
                      <option key={n} value={n}>
                        {n} 行
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  内边距
                  <input
                    type="number"
                    min={0}
                    max={32}
                    value={draft.terminalPadding}
                    onChange={(e) =>
                      setDraft((v) => ({
                        ...v,
                        terminalPadding: Number(e.target.value),
                      }))
                    }
                  />
                </label>
                <label className="km-check-label">
                  <input
                    type="checkbox"
                    checked={draft.terminalOptions.cursorBlink}
                    onChange={(e) =>
                      patchOption("cursorBlink", e.target.checked)
                    }
                  />
                  光标闪烁
                </label>
                <button className="km-primary" disabled={saving} type="submit">
                  {saving ? "保存中…" : "保存设置"}
                </button>
              </form>
            ) : (
              <>
                <p>
                  点击收藏仅查看，填入后需自行按回车执行。请勿保存密码或 Token。
                </p>
                <div className="km-saved-command-list">
                  {preferences.savedCommands.length === 0 && (
                    <p>尚未保存常用命令</p>
                  )}
                  {preferences.savedCommands.map((c) => (
                    <button
                      key={c.id}
                      aria-pressed={selected?.id === c.id}
                      onClick={() => {
                        setSelected(c);
                        setCommand(c);
                      }}
                    >
                      <strong>{c.name}</strong>
                      <small>{c.description || c.command}</small>
                    </button>
                  ))}
                </div>
                {selected && (
                  <div className="km-command-preview">
                    <pre>{selected.command}</pre>
                    <div className="km-command-actions">
                      <button
                        disabled={status !== "connected"}
                        onClick={() => {
                          api?.paste(selected.command);
                          setPanel(null);
                        }}
                      >
                        填入命令
                      </button>
                      <button
                        disabled={saving}
                        onClick={() => {
                          if (
                            window.confirm(`删除常用命令“${selected.name}”？`)
                          )
                            void save({
                              ...preferences,
                              savedCommands: preferences.savedCommands.filter(
                                (c) => c.id !== selected.id,
                              ),
                            }).then((ok) => {
                              if (ok) {
                                setSelected(null);
                                setCommand({
                                  id: "",
                                  name: "",
                                  command: "",
                                  description: "",
                                });
                              }
                            });
                        }}
                      >
                        删除
                      </button>
                    </div>
                  </div>
                )}
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    void saveCommand();
                  }}
                >
                  <h3>{command.id ? "编辑命令" : "新增命令"}</h3>
                  <label>
                    名称
                    <input
                      aria-label="名称"
                      required
                      maxLength={40}
                      value={command.name}
                      onChange={(e) =>
                        setCommand((v) => ({ ...v, name: e.target.value }))
                      }
                    />
                  </label>
                  <label>
                    命令内容
                    <textarea
                      aria-label="命令内容"
                      required
                      maxLength={8192}
                      rows={4}
                      value={command.command}
                      onChange={(e) =>
                        setCommand((v) => ({ ...v, command: e.target.value }))
                      }
                    />
                  </label>
                  <label>
                    说明
                    <input
                      aria-label="说明"
                      maxLength={160}
                      value={command.description}
                      onChange={(e) =>
                        setCommand((v) => ({
                          ...v,
                          description: e.target.value,
                        }))
                      }
                    />
                  </label>
                  <div className="km-command-actions">
                    <button
                      className="km-primary"
                      disabled={
                        saving ||
                        (!command.id && preferences.savedCommands.length >= 50)
                      }
                      type="submit"
                    >
                      保存命令
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setCommand({
                          id: "",
                          name: "",
                          command: "",
                          description: "",
                        });
                        setSelected(null);
                      }}
                    >
                      新建
                    </button>
                  </div>
                </form>
              </>
            )}
          </section>
        </div>
      )}
      <div className="km-terminal-accessory" aria-label="终端控制键">
        {[
          ["Esc", "\x1b"],
          ["Tab", "\t"],
          ["Ctrl+C", "\x03"],
          ["Ctrl+D", "\x04"],
          ["↑", "\x1b[A"],
          ["↓", "\x1b[B"],
          ["←", "\x1b[D"],
          ["→", "\x1b[C"],
        ].map(([label, value]) => (
          <button
            key={label}
            disabled={status !== "connected"}
            onClick={() => {
              api?.send(value);
              api?.terminal.focus();
            }}
          >
            {label}
          </button>
        ))}
      </div>
    </>
  );
}
