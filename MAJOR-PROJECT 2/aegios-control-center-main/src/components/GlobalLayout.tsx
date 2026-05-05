import { useState } from "react";
import { Outlet } from "react-router-dom";
import DashboardHeader from "./DashboardHeader";
import DashboardFooter from "./DashboardFooter";
import RightSidebar from "./security/RightSidebar";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { Activity } from "lucide-react";

const GlobalLayout = () => {
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false);

  return (
    <div className="min-h-screen bg-background">
      <DashboardHeader />
      
      {/* Main Layout Container */}
      <div className="flex">
        {/* Main Content Area */}
        <main className="flex-1 min-h-[calc(100vh-8rem)] lg:mr-4">
          <Outlet />
        </main>

        {/* Right Sidebar - Desktop Only (Static Flow) */}
        <aside 
          className="hidden lg:block w-80 bg-background border-l border-cyber-border/60 sticky top-16 self-start h-[calc(100vh-4rem)] overflow-y-auto"
          style={{ 
            boxShadow: '-2px 0 10px rgba(0, 255, 65, 0.06)'
          }}
          aria-label="Recent Activity Sidebar"
        >
          <div className="h-full overflow-hidden p-4">
            <RightSidebar />
          </div>
        </aside>

        {/* Mobile Sidebar Trigger Button */}
        <Sheet open={isMobileSidebarOpen} onOpenChange={setIsMobileSidebarOpen}>
          <SheetTrigger asChild>
            <Button
              variant="outline"
              size="icon"
              className="lg:hidden fixed bottom-6 right-6 z-50 neon-border bg-card hover:bg-secondary shadow-lg"
              aria-label="Open Recent Activity"
            >
              <Activity className="h-4 w-4" />
            </Button>
          </SheetTrigger>
          <SheetContent 
            side="right" 
            className="w-80 bg-background border-l border-cyber-border p-4"
          >
            <RightSidebar />
          </SheetContent>
        </Sheet>
      </div>

      <DashboardFooter />
    </div>
  );
};

export default GlobalLayout;