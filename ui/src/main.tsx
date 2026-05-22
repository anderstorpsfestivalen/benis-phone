import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Route, Routes, Navigate } from "react-router-dom";
import "./styles/index.css";
import App from "./App";
import ConfigList from "./routes/ConfigList";
import ConfigEditor from "./routes/ConfigEditor";
import FilesPage from "./routes/FilesPage";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route path="/" element={<ConfigList />} />
          <Route path="/editor/:name" element={<ConfigEditor />} />
          <Route path="/files" element={<FilesPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </StrictMode>,
);
