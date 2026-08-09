import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ReviewDetailPage from "./page";
import * as api from "@/lib/api";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "doc-1" }),
  useRouter: () => ({ push }),
}));

vi.mock("@/lib/api", () => ({
  getDocument: vi.fn(),
  submitReview: vi.fn(),
}));

const sampleDoc: api.Document = {
  document_id: "doc-1",
  status: "PENDING_REVIEW",
  document_type: "invoice",
  classification_confidence: 0.95,
  overall_confidence: 0.6,
  fields: {
    vendor_name: { value: "Acme", confidence: 0.9 },
    total_amount: { value: null, confidence: 0 },
  },
  created_at: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  push.mockReset();
  vi.mocked(api.getDocument).mockReset();
  vi.mocked(api.submitReview).mockReset();
});

describe("ReviewDetailPage", () => {
  it("shows a loading state before the document arrives", () => {
    vi.mocked(api.getDocument).mockReturnValue(new Promise(() => {}));

    render(<ReviewDetailPage />);

    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("renders fields, flags a missing one, and prefills the correction inputs", async () => {
    vi.mocked(api.getDocument).mockResolvedValue(sampleDoc);

    render(<ReviewDetailPage />);

    expect(await screen.findByText("vendor_name")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Acme")).toBeInTheDocument();
    expect(screen.getByText("missing")).toBeInTheDocument();
  });

  it("approve submits no corrected fields and navigates back to the queue", async () => {
    vi.mocked(api.getDocument).mockResolvedValue(sampleDoc);
    vi.mocked(api.submitReview).mockResolvedValue({
      document_id: "doc-1",
      review_status: "APPROVED",
      reviewed_by: "u",
    });

    render(<ReviewDetailPage />);
    await screen.findByText("vendor_name");
    await userEvent.click(screen.getByRole("button", { name: "Approve" }));

    await waitFor(() =>
      expect(api.submitReview).toHaveBeenCalledWith("doc-1", "approve", undefined, undefined)
    );
    expect(push).toHaveBeenCalledWith("/review");
  });

  it("save corrections submits the edited field values", async () => {
    vi.mocked(api.getDocument).mockResolvedValue(sampleDoc);
    vi.mocked(api.submitReview).mockResolvedValue({
      document_id: "doc-1",
      review_status: "CORRECTED",
      reviewed_by: "u",
    });

    render(<ReviewDetailPage />);
    await screen.findByText("vendor_name");

    const input = screen.getByDisplayValue("Acme");
    await userEvent.clear(input);
    await userEvent.type(input, "Corrected Vendor");
    await userEvent.click(screen.getByRole("button", { name: "Save corrections" }));

    await waitFor(() =>
      expect(api.submitReview).toHaveBeenCalledWith(
        "doc-1",
        "correct",
        { vendor_name: "Corrected Vendor", total_amount: "" },
        undefined
      )
    );
  });

  it("shows an error and stays on the page when submission fails", async () => {
    vi.mocked(api.getDocument).mockResolvedValue(sampleDoc);
    vi.mocked(api.submitReview).mockRejectedValue(new Error("review submission failed: conflict"));

    render(<ReviewDetailPage />);
    await screen.findByText("vendor_name");
    await userEvent.click(screen.getByRole("button", { name: "Reject" }));

    expect(await screen.findByText("review submission failed: conflict")).toBeInTheDocument();
    expect(push).not.toHaveBeenCalled();
  });

  it("shows the load error when the document fails to load", async () => {
    vi.mocked(api.getDocument).mockRejectedValue(new Error("document not found"));

    render(<ReviewDetailPage />);

    expect(await screen.findByText("document not found")).toBeInTheDocument();
  });
});
