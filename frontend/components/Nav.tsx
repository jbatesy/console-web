import Link from "next/link";

export default function Nav() {
  return (
    <nav className="flex items-center gap-4 border-b border-white/10 px-4 py-2 text-sm">
      <Link href="/" className="font-semibold">
        console-web
      </Link>
      <Link href="/jobs/" className="text-[var(--accent)]">
        Jobs
      </Link>
    </nav>
  );
}
