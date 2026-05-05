import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Github } from "lucide-react";

const ProfileSidebar = () => {
  return (
    <Card className="h-fit border-cyber-border bg-card glow-border">
      <CardHeader className="pb-4">
        <div className="flex flex-col items-center gap-3">
          <Avatar className="h-20 w-20 border-2 border-primary items-center justify-center bg-transparent overflow-hidden">
            <AvatarFallback className="bg-transparent text-primary flex items-center justify-center h-full w-full">
              <img src="/aegios_logo.png" alt="Aegios Logo" className="h-full w-full object-cover" />
            </AvatarFallback>
          </Avatar>
        </div>
      </CardHeader>
      <CardContent className="space-y-4 text-center">
        <div>
          <p className="text-sm text-muted-foreground">Organization</p>
          <p className="text-base font-semibold text-primary">Aegios Security</p>
        </div>
        <div>
          <p className="text-sm text-muted-foreground">Role</p>
          <p className="text-base font-semibold text-primary">Security Admin</p>
        </div>
      </CardContent>
    </Card>
  );
};

export default ProfileSidebar;
