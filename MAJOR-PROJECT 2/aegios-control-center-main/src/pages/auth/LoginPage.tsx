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

const LoginPage = () => {
  const navigate = useNavigate();
  const { login } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!username || !password) {
      toast.error('Please fill in all fields');
      return;
    }

    setIsLoading(true);
    const success = await login(username, password);
    setIsLoading(false);

    if (success) {
      toast.success('Login successful!');
      navigate('/fetching-service/dashboard');
    } else {
      toast.error('Invalid credentials');
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
          <p className="text-muted-foreground">Sign in to your account</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
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
              <Label htmlFor="password" className="text-foreground">
                Password
              </Label>
              <Input
                id="password"
                type="password"
                placeholder="Enter your password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="bg-secondary border-cyber-border text-foreground"
                disabled={isLoading}
              />
            </div>

            <div className="flex justify-end">
              <Link
                to="/authentication/forgot-password"
                className="text-sm text-primary hover:text-primary/80 transition-colors"
              >
                Forgot Password?
              </Link>
            </div>

            <Button
              type="submit"
              className="w-full bg-primary hover:bg-primary/90 text-background transition-all duration-300 hover:scale-[1.02] hover:shadow-[0_0_20px_hsl(145,60%,40%,0.5)]"
              disabled={isLoading}
            >
              {isLoading ? 'Signing in...' : 'Sign In'}
            </Button>

            <div className="text-center text-sm text-muted-foreground">
              Don't have an account?{' '}
              <Link
                to="/authentication/signup"
                className="text-primary hover:text-primary/80 transition-colors"
              >
                Sign up
              </Link>
            </div>
          </form>
        </CardContent>
      </Card>
      </div>
    </>
  );
};

export default LoginPage;
