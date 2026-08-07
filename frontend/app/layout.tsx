"use client";

import "./globals.css";
import React, { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Layout from "@/components/Layout";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Created per mount rather than at module scope so the cache is not shared
  // across requests when the bundle is reused.
  const [queryClient] = useState(() => new QueryClient());

  return (
    // I18nProvider keeps <html lang> in step with the selected locale.
    <html lang="mn">
      <body>
        {/*
          Next's metadata export is a server-component API and this root is a
          client component, so the document title and description are rendered
          as elements instead. React 19 hoists <title>/<meta> into <head> from
          anywhere in the tree, which keeps the providers below untouched — the
          alternative, splitting the root into a server layout plus a client
          providers file, is a larger change than the metadata warrants.
        */}
        <title>Gerege Nexus</title>
        <meta
          name="description"
          content="Төрийн болон хувийн хэвшлийн байгууллагын үйлчилгээ, үйл ажиллагаа, систем, өгөгдлийг нэгтгэх модульт платформ."
        />
        <ThemeProvider>
          <I18nProvider>
            <QueryClientProvider client={queryClient}>
              <Layout>{children}</Layout>
            </QueryClientProvider>
          </I18nProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
