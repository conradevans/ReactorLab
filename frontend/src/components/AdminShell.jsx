import Brand from "./Brand"
import ProductNav from "./ProductNav"

const items = [
  ["overview", "/admin", "Overview"],
  ["system", "/admin/system", "System"],
  ["deployments", "/admin/deployments", "Deployments"],
  ["databases", "/admin/databases", "Databases"],
  ["activity", "/admin/activity", "Activity"],
]

export default function AdminShell({ active, navigate, children }) {
  function go(path) {
    return (event) => {
      event.preventDefault()
      navigate(path)
    }
  }

  return (
    <main className="admin-page">
      <div className="app-shell">
        <header className="topbar">
          <Brand navigate={navigate} />

          <div className="header-actions">
            <ProductNav mode="admin" />
            <div className="control-plane-state">
              <span className="status-dot status-ready" />
              <span>
                <small>REACTORLAB</small>
                <strong>Foundation online</strong>
              </span>
            </div>
          </div>
        </header>

        <div className="admin-grid">
          <aside className="sidebar">
            <a className="nav-item switch-access" href="/" onClick={go("/")}>
              ← Switch access
            </a>

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
