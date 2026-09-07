export default function Brand({ navigate, subtitle = "Infrastructure control center" }) {
  function openHome(event) {
    event.preventDefault()
    navigate("/")
  }

  return (
    <a className="brand-link" href="/" onClick={openHome}>
      <span className="brand-mark">R</span>
      <span>
        <strong className="brand-name">ReactorLab</strong>
        <span className="brand-subtitle">{subtitle}</span>
      </span>
    </a>
  )
}
