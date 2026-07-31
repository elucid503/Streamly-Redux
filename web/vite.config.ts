import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath } from "node:url";

const publicHost = "streamly-redux.sprout.software";

const backend = "http://localhost:8080";

export default defineConfig({

  plugins: [react(), tailwindcss()],

  resolve: {

    alias: {

      "@": fileURLToPath(new URL("./src", import.meta.url)),

    },

  },

  server: {

    port: 9090,
    allowedHosts: [publicHost],

    hmr: {

      protocol: "wss",
      host: publicHost,
      clientPort: 443,

    },

    proxy: {

      "/.proxy": backend,
      "/api": backend,
      "/proxy": backend,

    },

  },

});
