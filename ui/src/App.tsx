import { Link, NavLink, Outlet, useLocation } from "react-router-dom";

// Top nav: brand on the left, then two views — config (the IVR editor)
// and files (R2 file manager). The active section is determined by URL,
// not router state, so both /editor/:name and / count as "config".

export default function App() {
  const loc = useLocation();
  const inFiles = loc.pathname.startsWith("/files");

  return (
    <div className="min-h-screen bg-ink-black text-white flex flex-col">
      <header className="border-b border-shadow-grey px-6 py-3 flex items-center gap-4">
        <Link to="/" className="font-mono text-blue-slate hover:text-white">
          ATP IVR
        </Link>
        <span className="text-shadow-grey">/</span>
        <nav className="flex items-center gap-1">
          <NavTab to="/" active={!inFiles}>config</NavTab>
          <NavTab to="/files" active={inFiles}>files</NavTab>
        </nav>
      </header>
      <main className="flex-1 p-3">
        <Outlet />
      </main>
    </div>
  );
}

function NavTab({
  to,
  active,
  children,
}: {
  to: string;
  active: boolean;
  children: React.ReactNode;
}) {
  return (
    <NavLink
      to={to}
      className={`px-3 py-1 text-xs font-mono rounded ${
        active
          ? "bg-blue-slate text-white"
          : "text-blue-slate hover:text-white border border-shadow-grey"
      }`}
    >
      {children}
    </NavLink>
  );
}
