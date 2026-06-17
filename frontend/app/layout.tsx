import type { Metadata } from "next";
import "@xterm/xterm/css/xterm.css";
import "./globals.css";
import Nav from "@/components/Nav";

export const metadata: Metadata = {
  title: "console-web",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <Nav />
        {children}
      </body>
    </html>
  );
}
