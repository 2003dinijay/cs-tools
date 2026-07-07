import React from "react";
import { createRoot } from "react-dom/client";

const container = document.getElementById("root");
if (!container) throw new Error("'Root container missing'");
const root = createRoot(container);
root.render(
  <React.StrictMode>
    <div>CSM Microapp</div>
  </React.StrictMode>,
);
