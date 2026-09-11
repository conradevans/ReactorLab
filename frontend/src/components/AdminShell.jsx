import { useEffect, useState } from "react"

import { getJSON } from "../api"
import GlobalHeader from "./GlobalHeader"

const items = [
  ["overview", "/admin", "Overview"],
  ["system", "/admin/system", "System"],
  ["deployments", "/admin/deployments", "Deployments"],
  ["databases", "/admin/databases", "Databases"],
  ["activity", "/admin/activity", "Activity"],
]

export default function AdminShell({ active, navigate, children }) {
  const [sessionLabel, setSessionLabel] = useState("Admin")

  useEffect(() => {
    let cancelled = false

    async function loadSession() {
      try {
        const session = await getJSON("/api/v1/session")
        if (!cancelled && session?.mode === "access" && session.email) {
          setSessionLabel(`Admin · ${session.email}`)
        }
      } catch {
        if (!cancelled) setSessionLabel("Admin")
      }
    }

    void loadSession()
    return () => {
      cancelled = true
    }
  }, [])

  function go(path) {
    return (event) => {
      event.preventDefault()
      navigate(path)
    }
  }

  return (
    <main className="admin-page">
      <div className="app-shell">
        <GlobalHeader
          mode="admin"
          navigate={navigate}
          sessionLabel={sessionLabel}
        />

        <div className="admin-grid">
          <aside className="sidebar">
            <nav aria-label="ReactorLab navigation">
              {items.map(([key, href, label]) => (
                <a
                  key={key}
                  className={active === key ? "nav-item active" : "nav-item"}
                  href={href}
                  onClick={go(href)}
                >
                  {label}
                </a>
              ))}
            </nav>

            <p className="sidebar-note">
              Administrator monitoring will be protected by Cloudflare Access
              before the public origin is enabled.
            </p>
          </aside>

          <div className="admin-content">{children}</div>
        </div>

        <footer>ReactorLab · Administrator dashboard</footer>
      </div>
    </main>
  )
}
