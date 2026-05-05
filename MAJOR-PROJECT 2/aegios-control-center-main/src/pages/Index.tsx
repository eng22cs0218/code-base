import ProfileSidebar from "@/components/ProfileSidebar";
import MainDashboard from "@/components/MainDashboard";

const Index = () => {
  return (
    <div className="grid grid-cols-[240px_minmax(0,1fr)] min-h-[calc(100vh-8rem)]">
      {/* Left Sidebar - Profile */}
      <aside className="pl-4 pr-0 py-6 sticky top-20 self-start border-r border-cyber-border/60" 
             style={{ boxShadow: '2px 0 10px rgba(0, 255, 65, 0.06)' }}>
        <ProfileSidebar />
      </aside>

      {/* Main Content Area */}
      <section className="px-6 py-6 min-w-0">
        <MainDashboard />
      </section>
    </div>
  );
};

export default Index;
