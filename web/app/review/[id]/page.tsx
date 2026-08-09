"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { getDocument, submitReview, type Document } from "@/lib/api";

export default function ReviewDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();

  const [doc, setDoc] = useState<Document | null>(null);
  const [edited, setEdited] = useState<Record<string, string>>({});
  const [notes, setNotes] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getDocument(id)
      .then((d) => {
        setDoc(d);
        const initial: Record<string, string> = {};
        for (const [name, f] of Object.entries(d.fields)) {
          initial[name] = f.value == null ? "" : String(f.value);
        }
        setEdited(initial);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "failed to load document"));
  }, [id]);

  async function decide(decision: "approve" | "reject" | "correct") {
    setBusy(true);
    setError(null);
    try {
      await submitReview(id, decision, decision === "correct" ? edited : undefined, notes || undefined);
      router.push("/review");
    } catch (err) {
      setError(err instanceof Error ? err.message : "review submission failed");
    } finally {
      setBusy(false);
    }
  }

  if (error && !doc) {
    return (
      <main>
        <p>
          <Link href="/review">&larr; Review queue</Link>
        </p>
        <p style={{ color: "crimson" }}>{error}</p>
      </main>
    );
  }

  if (!doc) return <main>Loading…</main>;

  return (
    <main style={{ maxWidth: 560 }}>
      <p>
        <Link href="/review">&larr; Review queue</Link>
      </p>
      <h1>Document {doc.document_id.slice(0, 8)}</h1>
      <p>
        Status: {doc.status} &middot; Type: {doc.document_type ?? "unknown"}
      </p>
      <p>
        Classification confidence: {doc.classification_confidence ?? "-"} &middot; Overall
        confidence: {doc.overall_confidence ?? "-"}
      </p>

      <h2>Fields</h2>
      <table style={{ borderCollapse: "collapse", width: "100%" }}>
        <tbody>
          {Object.entries(doc.fields).map(([name, f]) => (
            <tr key={name}>
              <td style={{ padding: 4, fontWeight: "bold" }}>{name}</td>
              <td style={{ padding: 4 }}>
                <input
                  value={edited[name] ?? ""}
                  onChange={(e) => setEdited({ ...edited, [name]: e.target.value })}
                  style={{ width: "100%" }}
                />
              </td>
              <td style={{ padding: 4, color: f.confidence === 0 ? "crimson" : undefined }}>
                {f.confidence === 0 ? "missing" : f.confidence.toFixed(2)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      <h2>Notes</h2>
      <textarea
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
        rows={3}
        style={{ width: "100%" }}
        placeholder="Optional notes for the audit log"
      />

      {error && <p style={{ color: "crimson" }}>{error}</p>}

      <div style={{ marginTop: 16, display: "flex", gap: 8 }}>
        <button onClick={() => decide("approve")} disabled={busy}>
          Approve
        </button>
        <button onClick={() => decide("correct")} disabled={busy}>
          Save corrections
        </button>
        <button onClick={() => decide("reject")} disabled={busy}>
          Reject
        </button>
      </div>
    </main>
  );
}
