import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Route, Routes, Navigate } from "react-router-dom";
import "./styles/index.css";
import App from "./App";

// Route-level code splitting. Each route ships its own JS chunk so the
// initial bundle stays small — ConfigEditor pulls in @xyflow/react + dagre
// (~150 kB), and FilesPage pulls in react-dropzone, neither of which is
// needed until the user navigates there.
const ConfigList = lazy(() => import("./routes/ConfigList"));
const ConfigEditor = lazy(() => import("./routes/ConfigEditor"));
const FilesPage = lazy(() => import("./routes/FilesPage"));

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <Suspense fallback={<div className="p-6 text-blue-slate text-sm font-mono">loading…</div>}>
        <Routes>
          <Route element={<App />}>
            <Route path="/" element={<ConfigList />} />
            <Route path="/editor/:name" element={<ConfigEditor />} />
            <Route path="/files" element={<FilesPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  </StrictMode>,
);
