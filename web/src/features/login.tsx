import { useState } from "react";
import { AlertCircle, Blocks, GitBranch } from "lucide-react";

export function Login({ onLogin }: { onLogin: (username: string) => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  return (
    <main className="login">
      <div className="brand-row"><span className="mark">T</span><strong>Tala</strong></div>
      <section className="login-panel">
        <h1>Welcome to Tala</h1>
        <p>Use a local username for issue edits, comments, and agent coordination.</p>
        <label>Username</label>
        <input value={name} onChange={(event) => setName(event.target.value)} placeholder="e.g. jdoe_ops" />
        {error && <div className="field-error"><AlertCircle size={15} />{error}</div>}
        <button className="button primary" onClick={() => {
          const trimmed = name.trim();
          if (!trimmed) return setError("Username is required.");
          onLogin(trimmed);
        }}>Continue</button>
      </section>
      <div className="login-features">
        <span><GitBranch size={16} />Hierarchy planning</span>
        <span><Blocks size={16} />Blocker tracking</span>
      </div>
    </main>
  );
}
