import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App";

import "@/index.css";

const container = document.getElementById("root");

if (!container) {

  throw new Error("root element missing");

}

const root = createRoot(container, {

  onUncaughtError: (error, info) => {

    const message = error instanceof Error ? error.message : String(error);
    const stack = [error instanceof Error ? error.stack : "", info.componentStack].filter(Boolean).join("\n");

    void fetch("/.proxy/api/client-error", {

      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message, stack }),

    }).catch(() => undefined);

    window.setTimeout(() => showFatalError(container, message), 0);

  },

});

root.render(

  <StrictMode>

    <App />

  </StrictMode>,

);

function showFatalError(target: HTMLElement, message: string) {

  const panel = document.createElement("div");

  panel.style.cssText = "display:flex;height:100%;flex-direction:column;align-items:center;justify-content:center;gap:12px;padding:24px;text-align:center;color:#f4f4f5;background:#18181b;font:14px system-ui";

  const title = document.createElement("strong");
  title.textContent = "Streamly encountered an error";

  const detail = document.createElement("span");
  detail.style.cssText = "max-width:640px;color:#a1a1aa";
  detail.textContent = message;

  const reload = document.createElement("button");
  reload.style.cssText = "border:1px solid #3f3f46;border-radius:8px;padding:8px 14px;color:#18181b;background:#f4f4f5;cursor:pointer";
  reload.textContent = "Reload Activity";
  reload.addEventListener("click", () => window.location.reload());

  panel.append(title, detail, reload);
  target.replaceChildren(panel);

}
