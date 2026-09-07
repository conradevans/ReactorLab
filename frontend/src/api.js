export async function getJSON(path) {
  const response = await fetch(path, {
    headers: { Accept: "application/json" },
    cache: "no-store",
  })

  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`)
  }

  return response.json()
}
