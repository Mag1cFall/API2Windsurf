type WailsRuntime = {
  WindowMinimise: () => void
  Quit: () => void
}

function rt(): WailsRuntime | null {
  const w = window as unknown as { runtime?: WailsRuntime }
  return w.runtime ?? null
}

export const Runtime = {
  minimise() {
    rt()?.WindowMinimise()
  },
  quit() {
    rt()?.Quit()
  },
}
