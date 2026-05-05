import { Outlet } from "react-router-dom";
import LeftSidebar from "./LeftSidebar";

const SecurityLayout = () => {
  return (
    <div className="grid grid-cols-[240px_minmax(0,1fr)] min-h-[calc(100vh-8rem)]">
      {/* Left Sidebar - Flush Left */}
      <aside className="pl-4 pr-0 py-6 sticky top-20 self-start border-r border-cyber-border/60" 
             style={{ boxShadow: '2px 0 10px rgba(0, 255, 65, 0.06)' }}>
        <LeftSidebar />
      </aside>

      {/* Main Content Area */}
      <section className="px-6 py-6 min-w-0">
        <Outlet />
      </section>
    </div>
  );
};

export default SecurityLayout;