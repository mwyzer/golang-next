import { afterEach, describe, expect, it, vi } from "vitest";
import { getDocument, getReviewQueue, submitReview, uploadDocument } from "./api";

function mockFetchOnce(status: number, body: unknown) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  }) as unknown as typeof fetch;
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("api client", () => {
  it("sends the bearer token and returns parsed JSON on success", async () => {
    mockFetchOnce(200, { document_id: "abc", status: "UPLOADED" });

    const doc = await getDocument("abc");

    expect(doc.document_id).toBe("abc");
    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/documents/abc");
    expect((init as RequestInit).headers).toMatchObject({ Authorization: expect.stringMatching(/^Bearer /) });
  });

  it("throws the server-provided error message on failure", async () => {
    mockFetchOnce(404, { error: { code: "NOT_FOUND", message: "document not found" } });

    await expect(getDocument("missing")).rejects.toThrow("document not found");
  });

  it("falls back to a generic message when the error body isn't JSON", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => {
        throw new Error("not json");
      },
    }) as unknown as typeof fetch;

    await expect(getDocument("x")).rejects.toThrow("request failed with status 500");
  });

  it("uploadDocument posts a multipart form containing the file", async () => {
    mockFetchOnce(201, { document_id: "new-id", status: "UPLOADED" });
    const file = new File(["hello"], "invoice.pdf", { type: "application/pdf" });

    await uploadDocument(file);

    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/documents");
    expect((init as RequestInit).method).toBe("POST");
    const form = (init as RequestInit).body as FormData;
    expect(form.get("file")).toBe(file);
  });

  it("submitReview sends decision, corrected_fields, and notes as a JSON body", async () => {
    mockFetchOnce(200, { document_id: "abc", review_status: "CORRECTED", reviewed_by: "u1" });

    await submitReview("abc", "correct", { vendor_name: "Acme" }, "looks right");

    const [url, init] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/documents/abc/review");
    expect((init as RequestInit).method).toBe("POST");
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      decision: "correct",
      corrected_fields: { vendor_name: "Acme" },
      notes: "looks right",
    });
  });

  it("getReviewQueue requests the review-queue endpoint", async () => {
    mockFetchOnce(200, { review_queue: [] });

    await getReviewQueue();

    const [url] = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/review-queue");
  });
});
