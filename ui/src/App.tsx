import { Link, Outlet } from "react-router-dom";

export default function App() {
  return (
    <div className="min-h-screen bg-ink-black text-white flex flex-col">
      <header className="border-b border-shadow-grey px-6 py-3 flex items-center gap-4">
        <Link to="/" className="font-mono text-blue-slate hover:text-white">
          benis-pbx
        </Link>
        <span className="text-shadow-grey">/</span>
        <span className="text-sm text-blue-slate">config editor</span>
      </header>
      <main className="flex-1 p-6">
        <Outlet />
      </main>
    </div>
  );
}
