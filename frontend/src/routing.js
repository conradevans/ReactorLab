export function resolveRoute(pathname) {
  if (pathname === "/") return { screen: "landing" }
  if (pathname === "/guest") return { screen: "guest" }
  if (pathname === "/admin") return { screen: "overview" }
  if (pathname === "/admin/system") return { screen: "system" }
  if (pathname === "/admin/deployments") return { screen: "deployments" }
  if (pathname === "/admin/databases") return { screen: "databases" }
  if (pathname === "/admin/activity") return { screen: "activity" }
  return { screen: "not-found" }
}
