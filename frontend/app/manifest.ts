import type { MetadataRoute } from "next";

/**
 * What the browser needs before it will offer to install this as an app.
 *
 * The native macOS, Windows, iOS and Android clients are a separate line of
 * work; this is the same platform installed straight from the browser, with no
 * download and no store. On a desktop it lands in the dock or the taskbar and
 * opens in its own window; on Android it goes to the home screen. The pages it
 * serves are exactly the pages the browser serves, so nothing here forks.
 *
 * Next generates this at /manifest.webmanifest and adds the <link> for it.
 */
export default function manifest(): MetadataRoute.Manifest {
  return {
    // `id` is what the browser matches an installed copy against. It is pinned
    // rather than derived from start_url, so changing where the app opens does
    // not make every installed copy look like a different app.
    id: "/",
    name: "Gerege Nexus",
    short_name: "Nexus",
    description:
      "Төрийн болон хувийн хэвшлийн байгууллагын үйлчилгээ, үйл ажиллагаа, систем, өгөгдлийг нэгтгэх модульт платформ.",
    // Mongolian is the source language of this platform, so it is what the
    // launcher labels the icon in before the app itself has loaded a locale.
    lang: "mn",
    dir: "ltr",
    start_url: "/",
    scope: "/",
    display: "standalone",
    orientation: "any",
    categories: ["business", "government", "productivity"],
    // The splash the launcher paints before the first frame. White rather than
    // the brand blue: the icon is blue, and blue on blue loses its edges.
    background_color: "#f9fafc",
    theme_color: "#1869eb",
    icons: [
      { src: "/icons/app-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icons/app-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      // Separate artwork, not the same file relabelled. A launcher that crops
      // to a circle would otherwise cut the corners off the card and leave four
      // pale notches; this one bleeds to the edge and keeps the emblem inside
      // the safe zone.
      {
        src: "/icons/app-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
    // The screens people open first. A shortcut is a long-press on the icon,
    // so this is a small list of destinations rather than a second menu.
    shortcuts: [
      {
        name: "Баримт ба цахим гарын үсэг",
        short_name: "Баримт",
        url: "/documents",
        icons: [{ src: "/icons/app-192.png", sizes: "192x192" }],
      },
      {
        name: "Төрийн үйлчилгээ",
        short_name: "Үйлчилгээ",
        url: "/gov-services",
        icons: [{ src: "/icons/app-192.png", sizes: "192x192" }],
      },
      {
        name: "Харилцагчид",
        short_name: "Харилцагч",
        url: "/contacts",
        icons: [{ src: "/icons/app-192.png", sizes: "192x192" }],
      },
    ],
  };
}
