export default function PlaceholderPage({ eyebrow, title, copy }) {
  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <h1>{title}</h1>
          <p>{copy}</p>
        </div>
      </section>

      <section className="content-section">
        <div className="section-card">
          <h2>Coming next</h2>
          <p className="placeholder-copy">{copy}</p>
        </div>
      </section>
    </>
  )
}
