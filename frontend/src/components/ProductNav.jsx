const destinations = {
  root: [
    ["ReactorLab", "/"],
    ["MiniDeploy", "https://minideploy.reactorlab.dev/"],
    ["MiniBase", "https://minibase.reactorlab.dev/"],
  ],
  admin: [
    ["ReactorLab", "/admin"],
    ["MiniDeploy", "https://minideploy.reactorlab.dev/admin/"],
    ["MiniBase", "https://minibase.reactorlab.dev/admin"],
    ["MiniAI", "https://miniai.reactorlab.dev/admin"],
  ],
  guest: [
    ["ReactorLab", "/guest"],
    ["MiniDeploy", "https://minideploy.reactorlab.dev/guest/"],
    ["MiniBase", "https://minibase.reactorlab.dev/guest"],
  ],
}

export default function ProductNav({ mode = "root", onNavigate }) {
  return (
    <nav className="product-nav" aria-label="ReactorLab products">
      {(destinations[mode] || destinations.root).map(([name, href]) => (
        <a
          key={name}
          className={name === "ReactorLab" ? "product-link active" : "product-link"}
          href={href}
          aria-current={name === "ReactorLab" ? "page" : undefined}
          onClick={onNavigate}
        >
          {name}
        </a>
      ))}
    </nav>
  )
}
