import type { ReactNode } from "react";

export const metadata = {
  title: "AI Document Processing Agent",
  description: "Upload and review documents processed by the AI agent.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body style={{ fontFamily: "system-ui, sans-serif", margin: "2rem" }}>
        {children}
      </body>
    </html>
  );
}
