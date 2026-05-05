import { Shield } from "lucide-react";

const DashboardFooter = () => {
  return (
    <footer className="border-t border-cyber-border bg-card mt-6">
      <div className="container mx-auto px-4 py-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Brand */}
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <Shield className="h-5 w-5 text-primary" />
              <span className="text-lg font-bold text-primary ">Aegios</span>
            </div>
            <p className="text-xs text-primary">
              © 2026 Aegios Security. All rights reserved.
            </p>
          </div>

          {/* Certifications */}
          <div className="space-y-2">
            <h3 className="text-xs font-semibold text-foreground">Certifications</h3>
            <div className="flex flex-wrap gap-2">
              <div className="px-3 py-1 rounded bg-secondary border border-cyber-border text-xs text-primary">
                ISO 27001
              </div>
              <div className="px-3 py-1 rounded bg-secondary border border-cyber-border text-xs text-primary">
                SOC 2 Type II
              </div>
              <div className="px-3 py-1 rounded bg-secondary border border-cyber-border text-xs text-primary">
                GDPR Compliant
              </div>
            </div>
          </div>

          {/* Quick Links */}
          <div className="space-y-2">
            <h3 className="text-xs font-semibold text-foreground">Quick Links</h3>
            <nav className="flex flex-col gap-1">
              <a href="#" className="text-xs text-muted-foreground hover:text-primary transition-colors">
                About Us
              </a>
              <a href="#" className="text-xs text-muted-foreground hover:text-primary transition-colors">
                Contact
              </a>
              <a href="#" className="text-xs text-muted-foreground hover:text-primary transition-colors">
                Privacy Policy
              </a>
              <a href="#" className="text-xs text-muted-foreground hover:text-primary transition-colors">
                Terms of Service
              </a>
            </nav>
          </div>
        </div>
      </div>
    </footer>
  );
};

export default DashboardFooter;
