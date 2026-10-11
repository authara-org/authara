import { defineConfig } from "vite";

const publicHost = process.env.PUBLIC_URL
  ? new URL(process.env.PUBLIC_URL).hostname
  : "localhost";

export default defineConfig({
  base: "/spa/",
  server: {
    allowedHosts: [publicHost],
    proxy: {
      "/auth": process.env.AUTHARA_PROXY_TARGET || "http://localhost:3001",
    },
  },
});
