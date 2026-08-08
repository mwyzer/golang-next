"use client";

import { useState } from "react";
import { getDocument, uploadDocument, type Document } from "@/lib/api";

export default function UploadPage() {
  const [file, setFile] = useState<File | null>(null);
  const [document, setDocument] = useState<Document | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function handleUpload(e: React.FormEvent) {
    e.preventDefault();
    if (!file) return;

    setBusy(true);
    setError(null);
    try {
      const { document_id } = await uploadDocument(file);
      const doc = await getDocument(document_id);
      setDocument(doc);
    } catch (err) {
      setError(err instanceof Error ? err.message : "upload failed");
    } finally {
      setBusy(false);
    }
  }

  async function handleRefresh() {
    if (!document) return;
    try {
      setDocument(await getDocument(document.document_id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "refresh failed");
    }
  }

  return (
    <main style={{ maxWidth: 480 }}>
      <h1>Upload a document</h1>
      <p>Supported: PDF, PNG, JPG/JPEG. See docs/SRS.md for the full spec.</p>

      <form onSubmit={handleUpload}>
        <input
          type="file"
          accept="application/pdf,image/png,image/jpeg"
          onChange={(e) => setFile(e.target.files?.[0] ?? null)}
        />
        <button type="submit" disabled={!file || busy} style={{ marginLeft: 8 }}>
          {busy ? "Uploading…" : "Upload"}
        </button>
      </form>

      {error && <p style={{ color: "crimson" }}>{error}</p>}

      {document && (
        <section style={{ marginTop: 24 }}>
          <h2>Document {document.document_id}</h2>
          <p>Status: {document.status}</p>
          <p>Type: {document.document_type ?? "not yet classified"}</p>
          <button onClick={handleRefresh}>Refresh status</button>
        </section>
      )}
    </main>
  );
}
