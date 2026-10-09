"use client";

import { type FC, type ReactNode } from "react";
import { ThemeProvider } from "next-themes";

export const Providers: FC<{ children: ReactNode }> = ({ children }) => {
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      {children}
    </ThemeProvider>
  );
};
