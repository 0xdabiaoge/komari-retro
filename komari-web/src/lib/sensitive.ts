// Only a short-lived authorization time is kept in memory, never the OTP.
let authorizedUntil = 0;
let pending: Promise<void> | null = null;

export async function authorizeSensitiveAccess(): Promise<void> {
  if (Date.now() < authorizedUntil) return;
  pending ??= (async () => {
    const response = await fetch("/api/me", { credentials: "include", cache: "no-store" });
    if (!response.ok) throw new Error("Unable to verify your account");
    const me = await response.json();
    if (me["2fa_enabled"]) {
      const code = window.prompt("敏感操作需要二次验证，请输入 2FA 验证码（有效期 5 分钟）");
      if (!code) throw new Error("已取消二次验证");
      const result = await fetch("/api/admin/2fa/verify", {
        method: "POST", credentials: "include",
        headers: { "X-2FA-Code": code },
      });
      if (!result.ok) throw new Error("2FA 验证失败，请重试");
    }
    authorizedUntil = Date.now() + 4 * 60 * 1000;
  })();
  try { await pending; } finally { pending = null; }
}

export const authorizeFileAccess = authorizeSensitiveAccess;
