"use client";

import LanguageSwitch from "@/components/Language";
import ThemeSwitcher from "@/components/ThemeSwitcher";
import DarkModeToggle from "@/components/DarkModeToggle";
import SpaLink from "@/components/SpaLink";
import { Calculator } from "lucide-react";
import { Button } from "@/components/ui/button";
import { usePublicInfo } from "@/contexts/PublicInfoContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useTranslation } from "react-i18next";
import { dispatchOpenRemainingValueCalculatorEvent } from "@/lib/remainingValueEvents";
import BrandMark from "@/components/BrandMark";


const NavBar = () => {
  const { publicInfo } = usePublicInfo();
  const { guestDisplay } = useTheme();
  const { t } = useTranslation();

  return (
    <nav className="sticky top-0 z-50 w-full border-b bg-background/80 backdrop-blur-md supports-[backdrop-filter]:bg-background/60 transition-all duration-300 shadow-sm">
      <div className="container mx-auto flex h-16 md:h-20 items-center justify-between px-4">
        {/* Logo and Title */}
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <SpaLink href="/" className="flex min-w-0 items-center gap-3 hover:opacity-80 transition-all">
            <BrandMark className="h-9 w-9 shrink-0" />
            <span className="flex min-w-0 flex-col leading-tight">
              <span className="truncate text-xl md:text-2xl font-bold tracking-tight bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
                {publicInfo?.sitename || "Komari Retro"}
              </span>
              <span className="hidden text-[9px] font-semibold tracking-[0.22em] text-muted-foreground sm:block">
                RETRO EDITION
              </span>
            </span>
          </SpaLink>
        </div>

        {/* Actions */}
        <div className="flex shrink-0 items-center gap-2">
          <DarkModeToggle />
          <ThemeSwitcher />
          {guestDisplay.showPrice && guestDisplay.showExpiredAt && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-9 w-9"
              onClick={dispatchOpenRemainingValueCalculatorEvent}
            >
              <Calculator className="h-4 w-4" />
              <span className="sr-only">
                {t("remainingValue.title", { defaultValue: "Remaining Value Calculator" })}
              </span>
            </Button>
          )}
          <LanguageSwitch />

        </div>
      </div>
    </nav>
  );
};

export default NavBar;
