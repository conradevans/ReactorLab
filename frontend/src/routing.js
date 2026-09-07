export function resolveRoute(pathname) {
  if (pathname === "/") return { screen: "landing" }
  if (pathname === "/guest") return { screen: "guest" }
  if (pathname === "/admin") return { screen: "overview" }
  if (pathname === "/admin/system") return { screen: "system" }
  if (pathname === "/admin/deployments") return { screen: "deployments" }

  const deploymentPrefix = "/admin/deployments/"
  if (pathname.startsWith(deploymentPrefix)) {
    const encodedApp = pathname.slice(deploymentPrefix.length)
    if (encodedApp && !encodedApp.includes("/")) {
      try {
        return {
          screen: "deployment-detail",
          app: decodeURIComponent(encodedApp),
        }
      } catch {
        return { screen: "not-found" }
      }
    }
  }

  if (pathname === "/admin/databases") return { screen: "databases" }

  const databasePrefix = "/admin/databases/"
  if (pathname.startsWith(databasePrefix)) {
    const encodedID = pathname.slice(databasePrefix.length)
    if (encodedID && !encodedID.includes("/")) {
      try {
        return {
          screen: "database-detail",
          id: decodeURIComponent(encodedID),
        }
      } catch {
        return { screen: "not-found" }
      }
    }
  }

  if (pathname === "/admin/activity") return { screen: "activity" }
  return { screen: "not-found" }
}
