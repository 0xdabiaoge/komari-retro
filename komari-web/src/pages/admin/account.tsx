import React from "react";

import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { AccountProvider, useAccount } from "@/contexts/AccountContext";
import {
  Button,
  Flex,
  TextField,
} from "@radix-ui/themes";
import Loading from "@/components/loading";

const Account = () => {
  return (
    <AccountProvider>
      <InnerLayout />
    </AccountProvider>
  );
};

const InnerLayout = () => {
  const { t } = useTranslation();
  const { account, loading, error, refresh } = useAccount();
  const [usernameSaving, setUsernameSaving] = React.useState(false);
  const [passwordSaving, setPasswordSaving] = React.useState(false);
  if (loading) {
    return <Loading />;
  }
  if (error) {
    return <div>{error.message}</div>;
  }

  function handleSubmitUsernameChange(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUsernameSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        username: (event.currentTarget as HTMLFormElement).username.value,
      }),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to update username");
        }
        return response.json();
      })
      .then(() => {
        refresh();
        toast.success(t("common.updated_successfully"));
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setUsernameSaving(false);
      });
  }
  function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget as HTMLFormElement;
    const password = form.password.value;
    const password_repeat = form.password_repeat.value;
    if (!password || !password_repeat) {
      toast.error(t("account.password_empty_error"));
      return;
    }
    if (password !== password_repeat) {
      toast.error(t("account.password_mismatch_error"));
      return;
    }
    if (password.length < 8) {
      toast.error(t("account.password_too_short_error"));
      return;
    }
    if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(password)) {
      toast.error(t("account.password_strength_error"));
      return;
    }
    setPasswordSaving(true);
    fetch("/api/admin/update/user", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        uuid: account?.uuid,
        password: password,
      }),
    })
      .then(async (response) => {
        if (!response.ok) {
          const data = await response.json();
          throw new Error(data.message || "Failed to update password");
        }
        return response.json();
      })
      .then(() => {
        toast.success(t("common.updated_successfully"));
        setTimeout(() => {
          window.location.href = "/";
        }, 2000);
      })
      .catch((error) => {
        toast.error(error.message);
      })
      .finally(() => {
        setPasswordSaving(false);
      });
  }

  return (
    <Flex gap="4" direction="column" align="start">
      <Flex gap="4" direction="row" className="km-page-admin-account p-4" wrap="wrap">
        <Flex gap="2" direction="column" className="w-full">
          <label className="text-2xl font-bold">{t("account.title")}</label>
          <label className="text-lg">
            {t("account.greeting", { username: account?.username })}
          </label>
          <form
            className="km-account-profile-form flex gap-2 flex-col"
            onSubmit={handleSubmitUsernameChange}
          >
            <label className="font-bold" htmlFor="username">
              {t("account.change_username_title")}
            </label>

            <TextField.Root
              className="max-w-128"
              id="username"
              name="username"
              defaultValue={account?.username}
            ></TextField.Root>
            <div>
              <Button disabled={usernameSaving} type="submit">
                {t("account.change_username_button")}
              </Button>
            </div>
          </form>
          <form onSubmit={changePassword} className="km-account-password-form flex flex-col gap-2">
            <label className="font-bold" htmlFor="old_password">
              {t("account.change_password_title")}
            </label>
            <label htmlFor="password">{t("account.new_password")}</label>
            <TextField.Root
              className="max-w-128"
              id="password"
              name="password"
              type="password"
            ></TextField.Root>
            <label htmlFor="password_repeat">
              {t("account.new_password_repeat")}
            </label>
            <TextField.Root
              className="max-w-128"
              id="password_repeat"
              name="password_repeat"
              type="password"
            ></TextField.Root>
            <div>
              <Button disabled={passwordSaving} type="submit">
                {t("account.change_password_button")}
              </Button>
            </div>
          </form>
        </Flex>
        <Flex direction="column" className="km-account-settings gap-2">
          <Flex gap="4" align="center" justify="start">
            <label className="text-muted-foreground text-sm">
              {t("account_settings.looking_for_backup")}
            </label>
          </Flex>
        </Flex>
      </Flex>
    </Flex>
  );
};
export default Account;
