const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const API_TOKEN = process.env.NEXT_PUBLIC_API_TOKEN ?? "dev-token";

export type ExtractedFieldValue = { value: unknown; confidence: number };

export type Document = {
  document_id: string;
  status: string;
  document_type: string | null;
  classification_confidence: number | null;
  overall_confidence: number | null;
  fields: Record<string, ExtractedFieldValue>;
  created_at: string;
};

export type ReviewQueueItem = {
  review_task_id: string;
  document_id: string;
  document_type: string | null;
  reason: string;
  classification_confidence: number | null;
  overall_confidence: number | null;
  created_at: string;
};

export type ReviewDecision = "approve" | "reject" | "correct";

export type ApiError = { error: { code: string; message: string } };

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}/api/v1${path}`, {
    ...init,
    headers: {
      Authorization: `Bearer ${API_TOKEN}`,
      ...init?.headers,
    },
  });

  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ApiError | null;
    throw new Error(body?.error?.message ?? `request failed with status ${res.status}`);
  }

  return res.json() as Promise<T>;
}

export function uploadDocument(file: File): Promise<{ document_id: string; status: string }> {
  const form = new FormData();
  form.append("file", file);
  return request("/documents", { method: "POST", body: form });
}

export function getDocument(id: string): Promise<Document> {
  return request(`/documents/${id}`);
}

export function getReviewQueue(): Promise<{ review_queue: ReviewQueueItem[] }> {
  return request("/review-queue");
}

export function submitReview(
  documentId: string,
  decision: ReviewDecision,
  correctedFields?: Record<string, unknown>,
  notes?: string
): Promise<{ document_id: string; review_status: string; reviewed_by: string }> {
  return request(`/documents/${documentId}/review`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      decision,
      corrected_fields: correctedFields,
      notes,
    }),
  });
}
