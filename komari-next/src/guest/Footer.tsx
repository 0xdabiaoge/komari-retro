"use client";

import BrandMark from "@/components/BrandMark";

export default function Footer() {
  return (
    <footer className="border-t bg-card/30">
      <div className="container mx-auto flex items-center justify-center gap-2 px-4 py-6 text-sm text-muted-foreground">
        <BrandMark className="h-6 w-6" />
        <span>实时监控</span>
      </div>
    </footer>
  );
}
