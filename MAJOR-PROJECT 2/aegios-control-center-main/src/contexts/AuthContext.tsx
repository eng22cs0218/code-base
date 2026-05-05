import React, { createContext, useContext, useState, useEffect } from 'react';
import { API_CONFIG } from '@/config/api';
import { setSessionToken, clearSessionToken, getSessionToken } from '@/lib/data-transformers';
import { toast } from 'sonner';

interface User {
  username: string;
  organization?: string;
  org_id?: string;
  cred_id?: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<boolean>;
  signup: (organization: string, username: string, token: string, password: string) => Promise<boolean>;
  logout: () => void;
  resetPassword: (usernameOrEmail: string) => Promise<boolean>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Check if user is already logged in
    const storedUser = localStorage.getItem('aegios_user');
    const sessionToken = getSessionToken();
    
    console.log('🔍 Checking existing session...');
    console.log('   Stored user:', storedUser ? 'Found' : 'Not found');
    console.log('   Session token:', sessionToken ? sessionToken.substring(0, 20) + '...' : 'Not found');
    
    if (storedUser && sessionToken) {
      setUser(JSON.parse(storedUser));
      setIsAuthenticated(true);
      console.log('✅ Session restored');
      
      // Trigger security data refresh
      window.dispatchEvent(new Event('aegios:login'));
    } else {
      console.log('⚠️ No existing session');
    }
    
    setIsLoading(false);
  }, []);

  const login = async (username: string, password: string): Promise<boolean> => {
    try {
      console.log('🔐 Attempting login...');
      console.log('📍 API URL:', API_CONFIG.ENDPOINTS.AUTH.LOGIN);
      
      const response = await fetch(API_CONFIG.ENDPOINTS.AUTH.LOGIN, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          github_username: username,
          password: password,
        }),
      });

      console.log('📥 Login response status:', response.status);

      const result = await response.json();
      console.log('📦 Login response:', result);

      if (!result.success) {
        console.error('❌ Login failed:', result.message || result.error);
        toast.error(result.message || result.error || 'Login failed');
        return false;
      }

      console.log('✅ Login successful:', result.data);
      
      // Store session token
      setSessionToken(result.data.session_token);
      console.log('💾 Session token stored:', result.data.session_token.substring(0, 20) + '...');
      
      // Store user data
      const userData: User = {
        username: result.data.github_username,
        organization: result.data.organization_name,
        org_id: result.data.org_id,
        cred_id: result.data.cred_id,
      };
      
      setUser(userData);
      setIsAuthenticated(true);
      localStorage.setItem('aegios_user', JSON.stringify(userData));
      console.log('💾 User data stored:', userData);
      
      // Trigger security data refresh
      console.log('🔄 Triggering security data refresh...');
      window.dispatchEvent(new Event('aegios:login'));
      
      toast.success(result.message || 'Login successful');
      return true;
    } catch (error) {
      console.error('❌ Login error:', error);
      toast.error('Login failed. Please try again.');
      return false;
    }
  };

  const signup = async (
    organization: string,
    username: string,
    token: string,
    password: string
  ): Promise<boolean> => {
    try {
      console.log('📝 Attempting signup...');
      console.log('📍 API URL:', API_CONFIG.ENDPOINTS.AUTH.SIGNUP);
      
      const response = await fetch(API_CONFIG.ENDPOINTS.AUTH.SIGNUP, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          organization_name: organization,
          github_username: username,
          pat: token,
          password: password,
        }),
      });

      const result = await response.json();
      console.log('📦 Signup response:', result);

      if (!result.success) {
        console.error('❌ Signup failed:', result.message || result.error);
        toast.error(result.message || result.error || 'Signup failed');
        return false;
      }

      console.log('✅ Signup successful:', result.data);
      toast.success(result.data.message || 'Account created successfully! Please login.');
      return true;
    } catch (error) {
      console.error('❌ Signup error:', error);
      toast.error('Signup failed. Please try again.');
      return false;
    }
  };

  const logout = async () => {
    const sessionToken = getSessionToken();
    
    // Synchronously clear local state before any network awaits that could be interrupted by redirects
    setUser(null);
    setIsAuthenticated(false);
    clearSessionToken();
    localStorage.removeItem('aegios_user');
    sessionStorage.removeItem('aegios_recent_activities');
    
    if (sessionToken) {
      try {
        console.log('🚪 Attempting logout...');
        console.log('📍 API URL:', API_CONFIG.ENDPOINTS.AUTH.SIGNOUT);
        
        const response = await fetch(API_CONFIG.ENDPOINTS.AUTH.SIGNOUT, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ session_token: sessionToken }),
        });

        const result = await response.json();
        console.log('📦 Logout response:', result);
        
        if (result.success) {
          console.log('✅ Logout successful');
        }
      } catch (error) {
        console.error('❌ Logout error:', error);
      }
    }
    
    toast.success('Logged out successfully');
  };

  const resetPassword = async (usernameOrEmail: string): Promise<boolean> => {
    // Password reset not implemented in backend yet
    toast.info('Password reset feature coming soon');
    return false;
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated,
        isLoading,
        login,
        signup,
        logout,
        resetPassword,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};
