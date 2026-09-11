import { useEffect, useId, useRef, useState } from "react"

export default function InfoControl({ label, children }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const rootRef = useRef(null)

  useEffect(() => {
    if (!open) return undefined

    function closeOutside(event) {
      if (!rootRef.current?.contains(event.target)) setOpen(false)
    }

    function closeOnEscape(event) {
      if (event.key === "Escape") setOpen(false)
    }

    document.addEventListener("pointerdown", closeOutside)
    document.addEventListener("keydown", closeOnEscape)
    return () => {
      document.removeEventListener("pointerdown", closeOutside)
      document.removeEventListener("keydown", closeOnEscape)
    }
  }, [open])

  return (
    <span className={open ? "metric-info open" : "metric-info"} ref={rootRef}>
      <button
        type="button"
        className="metric-info-button"
        aria-label={`About ${label}`}
        aria-describedby={id}
        aria-expanded={open}
        onClick={(event) => {
          event.stopPropagation()
          setOpen((current) => !current)
        }}
      >
        i
      </button>
      <span className="metric-popover" id={id} role="tooltip">
        {children}
      </span>
    </span>
  )
}
