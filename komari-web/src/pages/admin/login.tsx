import { useEffect, useState } from "react";
import { Button, Card, Flex, Heading, Text, TextField } from "@radix-ui/themes";
import { LoaderCircle, LogIn } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useSearchParams } from "react-router-dom";
import LanguageSwitch from "@/components/Language";
import ThemeSwitch from "@/components/ThemeSwitch";
import ColorSwitch from "@/components/ColorSwitch";
import { AccountProvider, useAccount } from "@/contexts/AccountContext";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import { useRPC2 } from "@/contexts/RPC2Context";
import Loading from "@/components/loading";
import { resolveLoginRedirect } from "@/utils/loginRedirect";

type LoginResponse = {
  message?: string;
};

const AdminLoginPage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { account, loading, refresh } = useAccount();
  const { client } = useRPC2();
  const { isLoading: publicInfoLoading } = usePublicInfo();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const redirect = resolveLoginRedirect(searchParams.get("redirect"));
  const passwordLoginEnabled = true;

  const canSubmit =
    passwordLoginEnabled && username.trim() !== "" && password !== "";

  useEffect(() => {
    if (!loading && account?.logged_in) {
      navigate(redirect, { replace: true });
    }
  }, [account, loading, navigate, redirect]);

  if (loading || publicInfoLoading) {
    return <Loading />;
  }
  if (account?.logged_in) {
    return <Navigate to={redirect} replace />;
  }

  const login = async () => {
    setBusy(true);
    setError("");
    try {
      const response = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username,
          password,
        }),
      });
      const payload = (await response.json()) as LoginResponse;
      if (!response.ok) {
        throw new Error(payload.message || `HTTP ${response.status}`);
      }
      // WebSocket cookies are fixed at the opening handshake. Replace the
      // pre-login socket before SPA navigation can issue administrative writes.
      client.disconnect();
      void client.connect().catch(() => {
        // HTTP RPC remains available if WebSocket reconnection fails.
      });
      await refresh();
      navigate(redirect, { replace: true });
    } catch (loginError) {
      setError(
        loginError instanceof Error
          ? loginError.message
          : t("login.request_failed"),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="km-login-page flex min-h-screen items-center justify-center bg-accent-1 p-4">
      <div className="absolute right-4 top-4 flex items-center gap-2">
        <ThemeSwitch />
        <ColorSwitch />
        <LanguageSwitch />
      </div>
      <Card className="km-login-card w-full max-w-[430px]">
        <Flex direction="column" gap="4">
          <div>
            <Heading size="6">{t("login.title")}</Heading>
            <Text as="p" size="2" color="gray" mt="1">
              {t("login.desc")}
            </Text>
          </div>
          <form
            className="km-login-form"
            onSubmit={(event) => {
              event.preventDefault();
              if (canSubmit && !busy) {
                void login();
              }
            }}
          >
            <Flex direction="column" gap="3">
              <>
                <label className="block">
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("login.username")}
                  </Text>
                  <TextField.Root
                    className="km-login-input"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    name="username"
                    autoComplete="username"
                    placeholder="admin"
                    disabled={busy}
                    autoFocus
                  />
                </label>
                <label className="block">
                  <Text as="div" size="2" weight="bold" mb="1">
                    {t("login.password")}
                  </Text>
                  <TextField.Root
                    className="km-login-input"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    name="password"
                    type="password"
                    autoComplete="current-password"
                    placeholder={t("login.password_placeholder")}
                    disabled={busy}
                  />
                </label>
                {error && (
                  <Text as="div" size="2" color="red" className="km-login-error">
                    {error}
                  </Text>
                )}
                <Button
                  type="submit"
                  className="w-full"
                  disabled={busy || !canSubmit}
                >
                  {busy ? (
                    <LoaderCircle size={16} className="animate-spin" />
                  ) : (
                    <LogIn size={16} />
                  )}
                  {busy ? t("loading") : t("login.title")}
                </Button>
                </>

            </Flex>
          </form>
        </Flex>
      </Card>
    </div>
  );
};

const AdminLogin = () => (
  <AccountProvider>
    <AdminLoginPage />
  </AccountProvider>
);

export default AdminLogin;
