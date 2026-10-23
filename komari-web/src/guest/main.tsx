import React, { Suspense, useMemo } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, useRoutes } from "react-router-dom";
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import "../global.css";
import "../i18n/config";
import { ThemeContext, THEME_DEFAULTS, type Appearance, type Colors } from "../contexts/ThemeContext";
import { useLocalStorage } from "../hooks/useLocalStorage";
import { useSystemTheme } from "../hooks/useSystemTheme";
import { RPC2Provider } from "../contexts/RPC2Context";
import { PublicInfoProvider } from "../contexts/PublicInfoContext";
import { NodeListProvider } from "../contexts/NodeListContext";
import ShareLayout from "../pages/share/_layout";
import ShareIndex from "../pages/share/index";
import Loading from "../components/loading";
import ErrorBoundary from "../components/ErrorBoundary";
import { Toaster } from "../components/ui/sonner";
const Instance = React.lazy(() => import("../pages/instance"));
function App() {
  const [appearance, setAppearance] = useLocalStorage<Appearance>("appearance", THEME_DEFAULTS.appearance);
  const [color, setColor] = useLocalStorage<Colors>("color", THEME_DEFAULTS.color);
  const resolved = useSystemTheme(appearance);
  const value = useMemo(() => ({ appearance, setAppearance, color, setColor }), [appearance, setAppearance, color, setColor]);
  const routes = useRoutes(["/", "/shared", "/s/:token"].map(path => ({
    path, element: <ShareLayout />, children: [{ index: true, element: <ShareIndex /> }, { path: "instance/:uuid", element: <Instance /> }],
  })));
  return <ThemeContext.Provider value={value}><Theme appearance={resolved} accentColor={color} scaling="110%" className="theme-root">
    <RPC2Provider><PublicInfoProvider><NodeListProvider><Toaster /><Suspense fallback={<Loading />}>{routes}</Suspense></NodeListProvider></PublicInfoProvider></RPC2Provider>
  </Theme></ThemeContext.Provider>;
}
createRoot(document.getElementById("root")!).render(<ErrorBoundary><BrowserRouter><App /></BrowserRouter></ErrorBoundary>);
