import { createRoot } from "react-dom/client";
import { useEffect, useState } from "react";

function App() {
  const [url, setUrl] = useState("");
  const [links, setLinks] = useState([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    const r = await fetch("/api/links");
    const j = await r.json();
    setLinks(j.links || []);
  }
  useEffect(() => { load(); }, []);

  async function submit(e) {
    e.preventDefault();
    setError(""); setBusy(true);
    try {
      const r = await fetch("/api/links", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url }),
      });
      const j = await r.json();
      if (!r.ok) throw new Error(j.error || "failed");
      setUrl("");
      await load();
    } catch (err) {
      setError(String(err.message || err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ fontFamily: "system-ui, sans-serif", maxWidth: 640, margin: "3rem auto", padding: "0 1rem" }}>
      <h1>linkbin</h1>
      <p style={{ color: "#666" }}>Paste a long URL, get a short one back.</p>
      <form onSubmit={submit} style={{ display: "flex", gap: 8 }}>
        <input
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://example.com/a/very/long/path"
          style={{ flex: 1, padding: 8 }}
          required
        />
        <button disabled={busy} type="submit">Shorten</button>
      </form>
      {error && <p style={{ color: "crimson" }}>{error}</p>}
      <ul style={{ marginTop: "2rem", paddingLeft: 0, listStyle: "none" }}>
        {links.map((l) => (
          <li key={l.code} style={{ padding: "8px 0", borderBottom: "1px solid #eee" }}>
            <a href={`/api/r/${l.code}`}>/r/{l.code}</a>
            {" -> "}
            <span style={{ color: "#666" }}>{l.url}</span>
            {" · "}
            <span style={{ color: "#999" }}>{l.clicks} click{l.clicks === 1 ? "" : "s"}</span>
          </li>
        ))}
        {links.length === 0 && <li style={{ color: "#999" }}>Nothing shortened yet.</li>}
      </ul>
    </div>
  );
}

createRoot(document.getElementById("root")).render(<App />);
