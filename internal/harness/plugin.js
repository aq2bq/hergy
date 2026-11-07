// hergy's opencode plugin (r-378e). hergy writes this directory into its own
// state directory and names it in OPENCODE_CONFIG_CONTENT's "plugins" when it
// starts an opencode member with --standalone; the person's ~/.config/opencode
// is not touched and is read as before.
// When a root session's execution ends, it calls `hergy turn-done <team>
// <member>` with the turn's text, so hergy's watcher hands the turn on without
// reading the pane.
import { spawn } from "node:child_process"

export default {
  id: "hergy.turns",
  async setup(ctx) {
    const bin = process.env.HERGY_BIN
    const team = process.env.HERGY_TEAM
    const member = process.env.HERGY_MEMBER
    if (!bin || !team || !member) return
    const texts = new Map()
    const controller = new AbortController()
    const isRoot = async (sessionID) => {
      try {
        const s = await ctx.session.get({ sessionID })
        const info = s && (s.data || s)
        return !(info && info.parentID)
      } catch {
        return true
      }
    }
    const send = (text) => {
      try {
        const child = spawn(bin, ["turn-done", team, member], { stdio: ["pipe", "ignore", "ignore"] })
        child.on("error", () => {})
        child.stdin.end(JSON.stringify({ source: "opencode", last_assistant_message: text }))
      } catch {}
    }
    ;(async () => {
      try {
        for await (const ev of ctx.event.subscribe({ signal: controller.signal })) {
          const d = ev.data || ev.properties || {}
          const id = d.sessionID
          if (!id) continue
          if (ev.type === "session.execution.started") {
            texts.set(id, [])
          } else if (ev.type === "session.text.ended") {
            texts.set(id, (texts.get(id) || []).concat(d.text || ""))
          } else if (ev.type.startsWith("session.execution.")) {
            const text = (texts.get(id) || []).join("\n")
            texts.delete(id)
            if (await isRoot(id)) send(text)
          }
        }
      } catch {}
    })()
    return () => controller.abort()
  },
}
