import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import UploadPage from "./page";
import * as api from "@/lib/api";

vi.mock("@/lib/api", () => ({
  uploadDocument: vi.fn(),
  getDocument: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(api.uploadDocument).mockReset();
  vi.mocked(api.getDocument).mockReset();
});

describe("UploadPage", () => {
  it("disables the upload button until a file is chosen", () => {
    render(<UploadPage />);

    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
  });

  it("uploads the selected file and shows the resulting document status", async () => {
    vi.mocked(api.uploadDocument).mockResolvedValue({ document_id: "doc-1", status: "UPLOADED" });
    vi.mocked(api.getDocument).mockResolvedValue({
      document_id: "doc-1",
      status: "UPLOADED",
      document_type: null,
      classification_confidence: null,
      overall_confidence: null,
      fields: {},
      created_at: "2026-01-01T00:00:00Z",
    });

    const { container } = render(<UploadPage />);
    const file = new File(["%PDF-1.4"], "invoice.pdf", { type: "application/pdf" });
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    await userEvent.upload(input, file);

    const uploadButton = screen.getByRole("button", { name: "Upload" });
    expect(uploadButton).toBeEnabled();
    await userEvent.click(uploadButton);

    expect(await screen.findByText("Status: UPLOADED")).toBeInTheDocument();
    expect(api.uploadDocument).toHaveBeenCalledWith(file);
  });

  it("shows an error message when the upload fails", async () => {
    vi.mocked(api.uploadDocument).mockRejectedValue(new Error("file exceeds the maximum allowed size"));

    const { container } = render(<UploadPage />);
    const file = new File(["x"], "big.pdf", { type: "application/pdf" });
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    await userEvent.upload(input, file);
    await userEvent.click(screen.getByRole("button", { name: "Upload" }));

    expect(await screen.findByText("file exceeds the maximum allowed size")).toBeInTheDocument();
  });
});
