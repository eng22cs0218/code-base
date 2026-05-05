import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Shield } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/contexts/AuthContext';
import { toast } from 'sonner';
import InteractiveBackground from '@/components/ui/interactive-background';

const SignupPage = () => {
  const navigate = useNavigate();
  const { signup } = useAuth();
  const [organization, setOrganization] = useState('');
  const [username, setUsername] = useState('');
  const [token, setToken] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!organization || !username || !token || !password) {
      toast.error('Please fill in all fields');
      return;
    }

    setIsLoading(true);
    const success = await signup(organization, username, token, password);
    setIsLoading(false);

    if (success) {
      toast.success('Account created successfully! Please sign in.');
      navigate('/authentication/login');
    } else {
      toast.error('Signup failed. Please try again.');
    }
  };

  return (
    <>
      <InteractiveBackground />
      <div className="min-h-screen flex items-center justify-center p-4 relative z-10">
        <Card className="w-full max-w-md border-cyber-border bg-card/80 backdrop-blur-md glow-border">
        <CardHeader className="text-center space-y-4">
          <div className="flex justify-center">
            <img src="/aegios_logo.png" alt="Aegios Logo" className="h-16 w-16 object-contain rounded-md" />
          </div>
          <CardTitle className="text-3xl font-bold text-primary glow-text">
            Aegios
          </CardTitle>
          <p className="text-muted-foreground">Create your account</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="organization" className="text-foreground">
                Organization Name
              </Label>
              <Input
                id="organization"
                type="text"
                placeholder="Enter your organization name"
                value={organization}
                onChange={(e) => setOrganization(e.target.value)}
                className="bg-secondary border-cyber-border text-foreground"
                disabled={isLoading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="username" className="text-foreground">
                GitHub Username
              </Label>
              <Input
                id="username"
                type="text"
                placeholder="Enter your GitHub username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="bg-secondary border-cyber-border text-foreground"
                disabled={isLoading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="token" className="text-foreground">
                GitHub Personal Access Token (PAT)
              </Label>
              <Input
                id="token"
                type="password"
                placeholder="Enter your GitHub PAT"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="bg-secondary border-cyber-border text-foreground"
                disabled={isLoading}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="password" className="text-foreground">
                Password
              </Label>
              <Input
                id="password"
                type="password"
                placeholder="Create a password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="bg-secondary border-cyber-border text-foreground"
                disabled={isLoading}
              />
            </div>

            <Button
              type="submit"
              className="w-full bg-primary hover:bg-primary/90 text-background transition-all duration-300 hover:scale-[1.02] hover:shadow-[0_0_20px_hsl(145,60%,40%,0.5)]"
              disabled={isLoading}
            >
              {isLoading ? 'Creating account...' : 'Sign Up'}
            </Button>

            <div className="text-center text-sm text-muted-foreground">
              Already have an account?{' '}
              <Link
                to="/authentication/login"
                className="text-primary hover:text-primary/80 transition-colors"
              >
                Sign in
              </Link>
            </div>
          </form>
        </CardContent>
      </Card>
      </div>
    </>
  );
};

export default SignupPage;
