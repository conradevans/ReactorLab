import { useCallback, useEffect, useState } from "react"

import AdminShell from "./components/AdminShell"
import GuestPage from "./components/GuestPage"
import LandingPage from "./components/LandingPage"
import OverviewPage from "./components/OverviewPage"
import PlaceholderPage from "./components/PlaceholderPage"
import SystemPage from "./components/SystemPage"
import { resolveRoute } from "./routing"

import "./App.css"

export default function App() {
  const [pathname, setPathname] = useState(() => window.location.pathname)

  useEffect(() => {
    const handler = () => setPathname(window.location.pathname)
    window.addEventListener("popstate", handler)
    return () => window.removeEventListener("popstate", handler)
  }, [])

  const navigate = useCallback((path) => {
    window.history.pushState({}, "", path)
    setPathname(path)
  }, [])

  const route = resolveRoute(pathname)

  if (route.screen === "landing") return <LandingPage navigate={navigate} />
  if (route.screen === "guest") return <GuestPage navigate={navigate} />

  const pages = {
    deployments: ["MINIDEPLOY", "Deployments", "Per-deployment and per-container CPU, RAM, network, disk, uptime, and restart metrics arrive in Phase 3."],
    databases: ["MINIBASE", "Databases", "Database storage, activity, connections, backups, and estimated CPU and RAM arrive in Phase 4."],
    activity: ["ACTIVITY", "Activity", "Unified MiniDeploy, MiniBase, and ReactorLab monitoring history arrives later in the monitoring build."],
  }

  if (route.screen === "overview") {
    return <AdminShell active="overview" navigate={navigate}><OverviewPage /></AdminShell>
  }

  if (route.screen === "system") {
    return <AdminShell active="system" navigate={navigate}><SystemPage /></AdminShell>
  }

  if (pages[route.screen]) {
    const [eyebrow, title, copy] = pages[route.screen]
    return (
      <AdminShell active={route.screen} navigate={navigate}>
        <PlaceholderPage eyebrow={eyebrow} title={title} copy={copy} />
      </AdminShell>
    )
  }

  return <LandingPage navigate={navigate} />
}
