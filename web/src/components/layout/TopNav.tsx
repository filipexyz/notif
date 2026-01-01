import { Link, useLocation } from "@tanstack/react-router";
import { UserButton } from "@clerk/tanstack-react-start";
import { Command } from "lucide-react";

const navItems = [
  { href: "/", label: "Events" },
  { href: "/webhooks", label: "Webhooks" },
  { href: "/dlq", label: "DLQ" },
  { href: "/settings", label: "Settings" },
];

interface TopNavProps {
  dlqCount?: number;
  onCommandPalette?: () => void;
}

export function TopNav({ dlqCount = 0, onCommandPalette }: TopNavProps) {
  const location = useLocation();

  return (
    <header className="h-12 border-b border-neutral-200 bg-white">
      <div className="h-full px-4 flex items-center justify-between">
        {/* Logo */}
        <div className="flex items-center gap-8">
          <Link to="/" className="text-lg font-semibold text-neutral-900">
            notif
          </Link>

          {/* Nav Links */}
          <nav className="flex items-center gap-1">
            {navItems.map((item) => {
              const isActive = location.pathname === item.href ||
                (item.href !== "/" && location.pathname.startsWith(item.href));

              return (
                <Link
                  key={item.href}
                  to={item.href}
                  className={`px-3 py-1.5 text-sm font-medium transition-colors ${
                    isActive
                      ? "text-primary-600 bg-primary-50"
                      : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50"
                  }`}
                >
                  {item.label}
                  {item.label === "DLQ" && dlqCount > 0 && (
                    <span className="ml-1.5 px-1.5 py-0.5 text-xs bg-error text-white">
                      {dlqCount}
                    </span>
                  )}
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Right side */}
        <div className="flex items-center gap-3">
          <button
            onClick={onCommandPalette}
            className="flex items-center gap-2 px-2 py-1 text-sm text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50 border border-neutral-200"
          >
            <Command className="w-3 h-3" />
            <span className="text-xs">K</span>
          </button>
          <UserButton afterSignOutUrl="/" />
        </div>
      </div>
    </header>
  );
}
