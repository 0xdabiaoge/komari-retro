import ThemeSwitch from "./ThemeSwitch";
import ColorSwitch from "./ColorSwitch";
import LanguageSwitch from "./Language";
import { Link, useParams } from "react-router-dom";
import { usePublicInfo } from "@/contexts/PublicInfoContext";

const ShareNavBar = () => {
  const { publicInfo } = usePublicInfo();
  const { token } = useParams<{ token: string }>();
  const homePath = token ? `/s/${token}` : "/";

  return (
    <nav className="km-navbar nav-bar flex rounded-b-lg items-center gap-2 md:gap-3 max-h-16 justify-end min-w-full p-2 px-4">
      <div className="km-navbar-brand mr-auto flex items-center min-w-0">
        <Link to={homePath} className="flex items-center min-w-0">
          <span className="font-bold text-[clamp(1.25rem,5vw,1.875rem)] whitespace-nowrap truncate leading-tight">
            {publicInfo?.sitename}
          </span>
        </Link>
        <div className="hidden flex-row items-baseline md:flex ml-3">
          <div
            style={{ borderColor: "var(--accent-3)" }}
            className="border-r-2 mr-2 h-4 self-center"
          />
          <span
            className="text-base font-bold whitespace-nowrap"
            style={{ color: "var(--accent-4)" }}
          >
            Komari Monitor
          </span>
        </div>
      </div>

      <div className="km-navbar-controls flex items-center gap-2 flex-shrink-0">
        <ThemeSwitch />
        <ColorSwitch />
        <LanguageSwitch />
      </div>
    </nav>
  );
};

export default ShareNavBar;
