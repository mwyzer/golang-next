"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { getReviewQueue, type ReviewQueueItem } from "@/lib/api";

export default function ReviewQueuePage() {
  const [items, setItems] = useState<ReviewQueueItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const { review_queue } = await getReviewQueue();
      setItems(review_queue);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load review queue");
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <main style={{ maxWidth: 720 }}>
      <p>
        <Link href="/">&larr; Upload</Link>
      </p>
      <h1>Review queue</h1>

      <button onClick={load}>Refresh</button>

      {error && <p style={{ color: "crimson" }}>{error}</p>}

      {items && items.length === 0 && <p>Nothing pending review.</p>}

      {items && items.length > 0 && (
        <table style={{ borderCollapse: "collapse", marginTop: 16, width: "100%" }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "1px solid #ccc" }}>
              <th style={{ padding: 8 }}>Document</th>
              <th style={{ padding: 8 }}>Type</th>
              <th style={{ padding: 8 }}>Reason</th>
              <th style={{ padding: 8 }}>Confidence</th>
              <th style={{ padding: 8 }}></th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.review_task_id} style={{ borderBottom: "1px solid #eee" }}>
                <td style={{ padding: 8, fontFamily: "monospace" }}>
                  {item.document_id.slice(0, 8)}
                </td>
                <td style={{ padding: 8 }}>{item.document_type ?? "unknown"}</td>
                <td style={{ padding: 8 }}>{item.reason}</td>
                <td style={{ padding: 8 }}>
                  {item.overall_confidence ?? item.classification_confidence ?? "-"}
                </td>
                <td style={{ padding: 8 }}>
                  <Link href={`/review/${item.document_id}`}>Review</Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  );
}
