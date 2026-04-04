import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider, createRouter } from "@tanstack/react-router";

import { routeTree } from "./routeTree.gen";
import "./app/globals.css";

const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  trailingSlash: "never",
  defaultNotFoundComponent: () => {
    const loc = window.location;
    return (
      <div style={{ padding: 40 }}>
        <h1>Route not found</h1>
        <p>Path: {loc.pathname}</p>
        <p>Full URL: {loc.href}</p>
      </div>
    );
  },
});

// Loose typing: accept any string for Link `to` prop.
// This is intentional — many links use dynamic paths like `/customers/${id}`.
// Type-safe routing can be adopted incrementally per-route later.
declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createRouter>;
  }
}

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>,
  );
}
