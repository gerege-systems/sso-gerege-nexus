"use client";

import "./globals.css";
import React, { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Layout from "@/components/Layout";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider } from "@/lib/theme";
import InstallApp from "@/components/InstallApp";

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
      <head>
        {/*
          Only what Next does not already emit.

          The manifest link and the viewport are its own — app/manifest.ts gets
          a <link> for free, and a default viewport is always written — so
          repeating either here put two of each in the served HTML.

          Everything below is Apple's, which reads none of the manifest: on iOS
          the icon, the name under it and the status bar are all decided by
          these tags instead.
        */}
        <meta name="theme-color" content="#1869eb" />
        <link rel="apple-touch-icon" href="/icons/apple-touch-icon.png" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
        <meta name="apple-mobile-web-app-title" content="Nexus" />
        <meta name="apple-mobile-web-app-status-bar-style" content="default" />
      </head>
      <body>
        {/*
          Next's metadata export is a server-component API and this root is a
          client component, so the document title and description are rendered
          as elements instead. React 19 hoists <title>/<meta> into <head> from
          anywhere in the tree, which keeps the providers below untouched — the
          alternative, splitting the root into a server layout plus a client
          providers file, is a larger change than the metadata warrants.
        */}
        <title>Gerege SSO</title>
        <meta
          name="description"
          content="Төрийн болон хувийн хэвшлийн байгууллагын үйлчилгээ, үйл ажиллагаа, систем, өгөгдлийг нэгтгэх модульт платформ."
        />
        <ThemeProvider>
          <I18nProvider>
            <QueryClientProvider client={queryClient}>
              <Layout>{children}</Layout>
              <InstallApp />
            </QueryClientProvider>
          </I18nProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
