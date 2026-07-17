import { defineConfig } from "vite";

export default defineConfig({
  base: "/spa/",
  server: {
    proxy: {
      "/auth": process.env.AUTHARA_PROXY_TARGET || "http://localhost:3001",
    },
  },
});
