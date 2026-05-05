import { Shield, User, Settings, Key, History, Bell, Download, HelpCircle, LogOut, Shield as ShieldIcon, BarChart3, FileText, Zap, Webhook, AlertTriangle, Database } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
  DropdownMenuLabel,
  DropdownMenuGroup,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { useNavigate, useLocation } from "react-router-dom";
import { toast } from "sonner";
import { useAuth } from "@/contexts/AuthContext";

const DashboardHeader = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { logout } = useAuth();

  const isSecurityPage = location.pathname.startsWith('/security');

  const handleProfileAction = (action: string) => {
    toast.info(`${action} - Feature coming soon!`);
  };

  const handleSignOut = () => {
    logout();
    toast.success("Signed out successfully");
    window.location.href = '/authentication/login';
  };

  return (
    <header className="sticky top-0 z-50 w-full border-b border-cyber-border bg-card backdrop-blur-sm">
      <div className="container mx-auto flex h-16 items-center justify-between px-4">
        {/* Logo and Brand */}
        <div className="flex items-center gap-2 cursor-pointer" onClick={() => navigate('/fetching-service/dashboard')}>
          <img src="/aegios_logo.png" alt="Aegios Logo" className="h-8 w-8 object-contain rounded-md" />
          <span className="text-2xl font-bold tracking-tight text-primary glow-text">
            Aegios
          </span>
        </div>

        {/* Navigation */}
        <nav className="flex items-center gap-6">
          <Button 
            variant="ghost" 
            onClick={() => navigate('/fetching-service/dashboard')}
            className={`text-foreground hover:text-[#00ff6a] hover:bg-secondary ${
              !isSecurityPage ? 'text-[#00ff6a] bg-secondary' : ''
            }`}
          >
            Dashboard
          </Button>
          
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button 
                variant="ghost" 
                className={`text-foreground hover:text-[#00ff6a] hover:bg-secondary ${
                  isSecurityPage ? 'text-[#00ff6a] bg-secondary' : ''
                }`}
              >
                Security
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="bg-card border-cyber-border">
              <DropdownMenuItem 
                className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                onClick={() => navigate('/security-service/k8s-score')}
              >
                K8s Score
              </DropdownMenuItem>
              <DropdownMenuItem 
                className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                onClick={() => navigate('/security-service/k8s-posture')}
              >
                K8s Posture
              </DropdownMenuItem>
              <DropdownMenuItem 
                className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                onClick={() => navigate('/security-service/k8s-action')}
              >
                K8s Actions
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="text-foreground hover:text-[#00ff6a] hover:bg-secondary flex items-center gap-2">
                <User className="h-4 w-4" />
                Profile
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64 bg-card border-cyber-border max-h-[80vh] overflow-y-auto">
              {/* User Info */}
              <DropdownMenuLabel className="text-[#00ff6a] sticky top-0 bg-card z-10">
                <div className="flex flex-col space-y-1">
                  <p className="text-sm font-medium">Aegios</p>
                  <p className="text-xs text-muted-foreground">Security Admin</p>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator className="bg-cyber-border" />

              {/* User Management */}
              <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("My Profile")}
                >
                  <User className="mr-2 h-4 w-4" />
                  My Profile
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Account Settings")}
                >
                  <Settings className="mr-2 h-4 w-4" />
                  Account Settings
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Security Settings")}
                >
                  <ShieldIcon className="mr-2 h-4 w-4" />
                  Security Settings
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Notifications")}
                >
                  <Bell className="mr-2 h-4 w-4" />
                  Notifications
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" />

              {/* Security-Specific Features */}
              <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("API Keys")}
                >
                  <Key className="mr-2 h-4 w-4" />
                  API Keys
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Access Logs")}
                >
                  <History className="mr-2 h-4 w-4" />
                  Access Logs
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Security Preferences")}
                >
                  <ShieldIcon className="mr-2 h-4 w-4" />
                  Security Preferences
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Compliance Reports")}
                >
                  <FileText className="mr-2 h-4 w-4" />
                  Compliance Reports
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" />

              {/* System & Preferences */}
              <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Theme Settings")}
                >
                  <Settings className="mr-2 h-4 w-4" />
                  Theme Settings
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Dashboard Layout")}
                >
                  <BarChart3 className="mr-2 h-4 w-4" />
                  Dashboard Layout
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Export Data")}
                >
                  <Download className="mr-2 h-4 w-4" />
                  Export Data
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Help & Support")}
                >
                  <HelpCircle className="mr-2 h-4 w-4" />
                  Help & Support
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" />

              {/* Security Dashboard */}
              {/* <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("My Security Score")}
                >
                  <ShieldIcon className="mr-2 h-4 w-4" />
                  My Security Score
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Recent Actions")}
                >
                  <History className="mr-2 h-4 w-4" />
                  Recent Actions
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Assigned Tasks")}
                >
                  <FileText className="mr-2 h-4 w-4" />
                  Assigned Tasks
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Incident Reports")}
                >
                  <AlertTriangle className="mr-2 h-4 w-4" />
                  Incident Reports
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" /> */}

              {/* Analytics & Reports */}
              {/* <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Usage Analytics")}
                >
                  <BarChart3 className="mr-2 h-4 w-4" />
                  Usage Analytics
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Performance Metrics")}
                >
                  <Zap className="mr-2 h-4 w-4" />
                  Performance Metrics
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Custom Reports")}
                >
                  <FileText className="mr-2 h-4 w-4" />
                  Custom Reports
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Audit Trail")}
                >
                  <History className="mr-2 h-4 w-4" />
                  Audit Trail
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" /> */}

              {/* Advanced Settings */}
              {/* <DropdownMenuGroup>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Integration Settings")}
                >
                  <Zap className="mr-2 h-4 w-4" />
                  Integration Settings
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Webhook Configuration")}
                >
                  <Webhook className="mr-2 h-4 w-4" />
                  Webhook Configuration
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Alert Rules")}
                >
                  <AlertTriangle className="mr-2 h-4 w-4" />
                  Alert Rules
                </DropdownMenuItem>
                <DropdownMenuItem 
                  className="cursor-pointer text-foreground hover:text-[#00ff6a] hover:bg-secondary"
                  onClick={() => handleProfileAction("Backup & Restore")}
                >
                  <Database className="mr-2 h-4 w-4" />
                  Backup & Restore
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator className="bg-cyber-border" /> */}

              {/* Session Management */}
              <DropdownMenuItem 
                className="cursor-pointer text-destructive hover:text-destructive hover:bg-destructive/10"
                onClick={handleSignOut}
              >
                <LogOut className="mr-2 h-4 w-4" />
                Sign Out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </nav>
      </div>
    </header>
  );
};

export default DashboardHeader;
