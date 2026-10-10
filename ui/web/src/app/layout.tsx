import "@/@assets/globals.css";

import { fontGeistMono, fontGeistSans } from "@/lib/font.lib";
import { Providers } from "@/providers";

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      data-theme="dark"
      className={`${fontGeistSans.variable} ${fontGeistMono.variable} h-full antialiased dark`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col bg-background text-foreground">
        <Providers>
          <main>{children}</main>
        </Providers>
      </body>
    </html>
  );
}
