import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ReviewQueuePage from "./page";
import * as api from "@/lib/api";

vi.mock("@/lib/api", () => ({
  getReviewQueue: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(api.getReviewQueue).mockReset();
});

describe("ReviewQueuePage", () => {
  it("shows an empty state when nothing is pending", async () => {
    vi.mocked(api.getReviewQueue).mockResolvedValue({ review_queue: [] });

    render(<ReviewQueuePage />);

    expect(await screen.findByText("Nothing pending review.")).toBeInTheDocument();
  });

  it("renders each queued item with its reason and a link to its detail page", async () => {
    vi.mocked(api.getReviewQueue).mockResolvedValue({
      review_queue: [
        {
          review_task_id: "rt1",
          document_id: "11111111-2222-3333-4444-555555555555",
          document_type: "invoice",
          reason: "LOW_CONFIDENCE",
          classification_confidence: 0.8,
          overall_confidence: 0.5,
          created_at: "2026-01-01T00:00:00Z",
        },
      ],
    });

    render(<ReviewQueuePage />);

    expect(await screen.findByText("LOW_CONFIDENCE")).toBeInTheDocument();
    expect(screen.getByText("invoice")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Review" })).toHaveAttribute(
      "href",
      "/review/11111111-2222-3333-4444-555555555555"
    );
  });

  it("shows an error message when loading fails", async () => {
    vi.mocked(api.getReviewQueue).mockRejectedValue(new Error("boom"));

    render(<ReviewQueuePage />);

    expect(await screen.findByText("boom")).toBeInTheDocument();
  });

  it("reloads the queue when Refresh is clicked", async () => {
    vi.mocked(api.getReviewQueue).mockResolvedValue({ review_queue: [] });

    render(<ReviewQueuePage />);
    await screen.findByText("Nothing pending review.");
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));

    expect(api.getReviewQueue).toHaveBeenCalledTimes(2);
  });
});
