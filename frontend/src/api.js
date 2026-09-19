export async function getJSON(path, { signal } = {}) {
  const response = await fetch(path, {
    headers: { Accept: "application/json" },
    cache: "no-store",
    ...(signal ? { signal } : {}),
  })

  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`)
  }

  return response.json()
}
