import * as React from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Flex } from "@/components/ui/flex";
import { Text } from "@/components/ui/text";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Box } from "@/components/ui/box";
import { useTranslation } from "react-i18next";
import { TablerSettings } from "./Icones/Tabler";
import { AccountProvider, useAccount } from "@/contexts/AccountContext";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import { useMounted } from "@/hooks/useMounted";

type LoginDialogProps = {
  trigger?: React.ReactNode | string;
  autoOpen?: boolean;
  showSettings?: boolean;
  info?: string | React.ReactNode;
  onLoginSuccess?: () => void;
};

const LoginDialogContent = ({
  trigger,
  autoOpen = false,
  showSettings = true,
  info,
  onLoginSuccess,
}: LoginDialogProps) => {
  const { account, loading, error, refresh } = useAccount();
  const {
    publicInfo,
    isLoading: publicInfoLoading,
    error: publicInfoError,
  } = usePublicInfo();
  const [t] = useTranslation();
  const mounted = useMounted();
  const [username, setUsername] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [errorMsg, setErrorMsg] = React.useState("");
  const [isLoading, setIsLoading] = React.useState(false);
  const [open, setOpen] = React.useState(autoOpen);

  const publicInfoReady = publicInfo !== null;
  const passwordLoginEnabled = publicInfoReady;

  const isFormValid =
    passwordLoginEnabled &&
    username.trim() !== "" &&
    password.trim() !== "";

  React.useEffect(() => {
    if (autoOpen) setOpen(true);
  }, [autoOpen]);

  const handleLogin = async () => {
    if (!isFormValid) {
      setErrorMsg("Username and password are required");
      return;
    }

    setErrorMsg("");
    setIsLoading(true);
    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          username,
          password,
        }),
      });
      const data = await res.json();
      if (res.status === 200) {
        refresh();
        if (typeof onLoginSuccess === "function") {
          onLoginSuccess();
          return;
        }
        window.open("/admin", "_self");
      } else {
        setErrorMsg(data.message || "Login failed");
      }
    } catch (err) {
      setErrorMsg("Network error");
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !isLoading && isFormValid) {
      e.preventDefault();
      handleLogin();
    }
  };

  const authStateLoading =
    loading ||
    publicInfoLoading ||
    (!publicInfoReady && !publicInfoError);

  if (authStateLoading) {
    return <Button disabled>{mounted ? t("loading") : "Loading..."}</Button>;
  }
  if (error || publicInfoError || !account || !publicInfoReady) {
    return (
      <Button disabled className="text-destructive">
        Error
      </Button>
    );
  }
  if (account.logged_in) {
    if (!showSettings) {
      return null;
    }
    return (
      <a href="/admin" target="_blank">
        <Button variant="ghost" size="icon">
          <TablerSettings />
        </Button>
      </a>
    );
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ? trigger : <Button>{t("login.title")}</Button>}
      </DialogTrigger>
      <DialogContent className="max-w-[450px]">
        <DialogTitle>{t("login.title")}</DialogTitle>
        <DialogDescription className="mb-4">
          <div className="flex justify-center flex-col gap-2">
            <label>{t("login.desc")}</label>
            {info && <label>{info}</label>}
          </div>
        </DialogDescription>
        <Box
          onSubmit={(e) => {
            e.preventDefault();
            if (isFormValid && !isLoading) {
              handleLogin();
            }
          }}
        >
          <Flex direction="column" gap="3">
            <>
                <label>
                  <Text as="div" size="2" weight="bold">
                    {t("login.username")}
                  </Text>
                  <Input
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="admin"
                    disabled={isLoading}
                    autoFocus
                  />
                </label>
                <label>
                  <Text as="div" size="2" weight="bold">
                    {t("login.password")}
                  </Text>
                  <Input
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    onKeyDown={handleKeyDown}
                    type="password"
                    placeholder={t("login.password_placeholder")}
                    disabled={isLoading}
                  />
                </label>
                {errorMsg && (
                  <Text as="div" size="2" className="text-destructive">
                    {errorMsg}
                  </Text>
                )}
                <Button
                  type="submit"
                  disabled={isLoading || !isFormValid}
                  style={{ opacity: isLoading || !isFormValid ? 0.6 : 1 }}
                  onClick={handleLogin}
                >
                  {isLoading ? "Logging in..." : t("login.title")}
                </Button>
              </>

          </Flex>
        </Box>
      </DialogContent>
    </Dialog>
  );
};

const LoginDialog = (props: LoginDialogProps) => {
  return (
    <AccountProvider>
      <LoginDialogContent {...props} />
    </AccountProvider>
  );
};

export default LoginDialog;
