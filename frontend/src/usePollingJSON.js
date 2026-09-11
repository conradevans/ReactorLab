import { useEffect, useState } from "react"

import { getJSON } from "./api"

export default function usePollingJSON(path, intervalMs = 5000) {
  const [state, setState] = useState({
    data: null,
    error: "",
    loading: true,
  })

  useEffect(() => {
    let cancelled = false
    let timeout

    async function load() {
      try {
        const data = await getJSON(path)
        if (!cancelled) {
          setState({ data, error: "", loading: false })
        }
      } catch {
        if (!cancelled) {
          setState((current) => ({
            ...current,
            error: "Unable to load live metrics.",
            loading: false,
          }))
        }
      }

      if (!cancelled) {
        timeout = window.setTimeout(load, intervalMs)
      }
    }

    void load()

    return () => {
      cancelled = true
      window.clearTimeout(timeout)
    }
  }, [path, intervalMs])

  return state
}
