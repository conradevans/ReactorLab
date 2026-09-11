import GlobalHeader from "./GlobalHeader"

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
        <GlobalHeader mode="root" navigate={navigate} />

        <section className="landing-hero">
          <div className="landing-copy">
            <p className="eyebrow">REACTORLAB CONTROL CENTER</p>
            <h1>Understand your self-hosted platform in one place.</h1>
            <p className="hero-copy">
              ReactorLab shows the Dell's system health, applications,
              databases, operational activity, and local diagnostics without
              depending on an external hosting control panel.
            </p>

            <dl className="access-explanation">
              <div>
                <dt>What it does</dt>
                <dd>Gives you one view of the server and the services and applications running on it.</dd>
              </div>
              <div>
                <dt>How it works</dt>
                <dd>Collects local system data and safe operational information from MiniDeploy and MiniBase.</dd>
              </div>
              <div>
                <dt>Why it is useful</dt>
                <dd>See what is running and whether the platform is healthy without jumping between terminal commands and services.</dd>
              </div>
            </dl>
          </div>

          <aside className="principles-card access-card">
            <p className="eyebrow">CHOOSE ACCESS</p>
            <h2>Open ReactorLab</h2>
            <p>
              Administrator access opens detailed infrastructure and platform
              monitoring. Guest access provides a restricted, read-only health
              overview.
            </p>
            <div className="access-actions">
              <a className="button primary" href="/admin">
                Open Administrator
              </a>
              <a className="button secondary" href="/guest" onClick={go("/guest")}>
                Guest Overview
              </a>
            </div>
          </aside>
        </section>

        <footer>ReactorLab · Dell infrastructure control center</footer>
      </div>
    </main>
  )
}
