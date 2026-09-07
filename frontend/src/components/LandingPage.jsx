import Brand from "./Brand"
import ProductNav from "./ProductNav"

export default function LandingPage({ navigate }) {
  function go(path) {
    return (event) => {
      event.preventDefault()
      navigate(path)
    }
  }

  return (
    <main className="public-page">
      <div className="site-shell">
        <header className="public-nav">
          <Brand navigate={navigate} />
          <ProductNav mode="root" />
        </header>

        <section className="landing-hero">
          <div className="landing-copy">
            <p className="eyebrow">REACTORLAB</p>
            <h1>Your Dell infrastructure at a glance.</h1>
            <p className="hero-copy">
              Monitor system health, deployments, databases, resource usage,
              and ReactorLab services from one control center.
            </p>

            <div className="hero-actions">
              <a className="button primary" href="/admin">
                Open administrator
              </a>
              <a className="button secondary" href="/guest" onClick={go("/guest")}>
                Guest overview
              </a>
            </div>
          </div>

          <div className="principles-card">
            <ul>
              <li>
                <strong>Dell monitoring</strong>
                <span>CPU, memory, disk, temperature, network, and uptime.</span>
              </li>
              <li>
                <strong>MiniDeploy visibility</strong>
                <span>Per-application and per-container resource usage.</span>
              </li>
              <li>
                <strong>MiniBase visibility</strong>
                <span>Database health, activity, storage, and backups.</span>
              </li>
              <li>
                <strong>MiniAI ready</strong>
                <span>Structured monitoring data for future diagnostics.</span>
              </li>
            </ul>
          </div>
        </section>

        <footer>ReactorLab · Dell infrastructure control center</footer>
      </div>
    </main>
  )
}
