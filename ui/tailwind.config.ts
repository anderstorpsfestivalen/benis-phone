import type { Config } from "tailwindcss";

// Palette override: only these five colors exist. Importing any default
// tailwind color (e.g. bg-red-500) will produce a build error — that's
// intentional. Edit this file to add a color; do not pull from defaults.
const config: Config = {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    colors: {
      transparent: "transparent",
      current: "currentColor",
      "ink-black": "#02111b",
      gunmetal: "#3f4045",
      "shadow-grey": "#30292f",
      "blue-slate": "#5d737e",
      white: "#fcfcfc",
    },
    extend: {
      fontFamily: {
        mono: [
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "Monaco",
          "Consolas",
          "monospace",
        ],
      },
    },
  },
  plugins: [],
};

export default config;
