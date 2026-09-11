export default function Brand({ navigate }) {
  function openHome(event) {
    event.preventDefault()
    navigate("/")
  }

  return (
    <a
      aria-label="ReactorLab home"
      className="brand-link"
      href="/"
      onClick={openHome}
    >
      <span className="brand-mark">R</span>
      <strong className="brand-name">ReactorLab</strong>
    </a>
  )
}
