import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const publicHost = "streamly-redux.sprout.software";

const backend = "http://localhost:8080";

export default defineConfig(({ command }) => {

  // Discord caches individual Vite modules independently; a new base makes every dev-server restart a clean session.
  const base = command === "serve" ? `/__streamly_dev_${Date.now()}/` : "/";

  return {

    base,

    plugins: [react(), tailwindcss()],

    resolve: {

      alias: {

        "@": fileURLToPath(new URL("./src", import.meta.url)),

      },

    },

    server: {

      port: 9090,
      allowedHosts: [publicHost],

      headers: {

        "Cache-Control": "no-store, max-age=0",
        "Expires": "0",
        "Pragma": "no-cache",

      },

      hmr: {

        protocol: "wss",
        host: publicHost,
        clientPort: 443,

      },

      proxy: {

        // Kept for legacy Discord proxy URLs; current clients connect directly through /ws.
        "/.proxy": { target: backend, ws: true },

        "/api": backend,
        "/proxy": backend,
        "/ws": { target: backend, ws: true },

      },

    },

  };

});
