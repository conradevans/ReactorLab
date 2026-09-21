import { useCallback, useEffect, useState } from "react"

import ActivityPage from "./components/ActivityPage"
import AdminShell from "./components/AdminShell"
import DatabaseDetailPage from "./components/DatabaseDetailPage"
import DatabasesPage from "./components/DatabasesPage"
import DeploymentDetailPage from "./components/DeploymentDetailPage"
import DeploymentsPage from "./components/DeploymentsPage"
import GuestPage from "./components/GuestPage"
import LandingPage from "./components/LandingPage"
import ObservabilityPage from "./components/ObservabilityPage"
import OverviewPage from "./components/OverviewPage"
import SystemPage from "./components/SystemPage"
import { resolveRoute } from "./routing"

import "./App.css"

function browserLocation() {
  return {
    pathname: window.location.pathname,
    search: window.location.search,
  }
}

export default function App() {
  const [location, setLocation] = useState(browserLocation)

  useEffect(() => {
    const handler = () => setLocation(browserLocation())
    window.addEventListener("popstate", handler)
    return () => window.removeEventListener("popstate", handler)
  }, [])

  const navigate = useCallback((path) => {
    window.history.pushState({}, "", path)
    setLocation(browserLocation())
  }, [])

  const route = resolveRoute(location.pathname)
  const activityEventID =
    new URLSearchParams(location.search).get("event") || ""

  if (route.screen === "landing") return <LandingPage navigate={navigate} />
  if (route.screen === "guest") return <GuestPage navigate={navigate} />

  if (route.screen === "activity") {
    return (
      <AdminShell active="activity" navigate={navigate}>
        <ActivityPage eventID={activityEventID} />
      </AdminShell>
    )
  }

  if (route.screen === "overview") {
    return (
      <AdminShell active="overview" navigate={navigate}>
        <OverviewPage navigate={navigate} />
      </AdminShell>
    )
  }

  if (route.screen === "system") {
    return (
      <AdminShell active="system" navigate={navigate}>
        <SystemPage />
      </AdminShell>
    )
  }

  if (route.screen === "observability") {
    return (
      <AdminShell active="observability" navigate={navigate}>
        <ObservabilityPage />
      </AdminShell>
    )
  }

  if (route.screen === "deployments") {
    return (
      <AdminShell active="deployments" navigate={navigate}>
        <DeploymentsPage navigate={navigate} />
      </AdminShell>
    )
  }

  if (route.screen === "deployment-detail") {
    return (
      <AdminShell active="deployments" navigate={navigate}>
        <DeploymentDetailPage app={route.app} navigate={navigate} />
      </AdminShell>
    )
  }

  if (route.screen === "databases") {
    return (
      <AdminShell active="databases" navigate={navigate}>
        <DatabasesPage navigate={navigate} />
      </AdminShell>
    )
  }

  if (route.screen === "database-detail") {
    return (
      <AdminShell active="databases" navigate={navigate}>
        <DatabaseDetailPage id={route.id} navigate={navigate} />
      </AdminShell>
    )
  }

  return <LandingPage navigate={navigate} />
}
