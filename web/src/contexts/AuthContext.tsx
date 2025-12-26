import { createContext, useContext, useState } from 'react';
import type { ReactNode } from 'react';

type UserRole = 'super_admin' | 'org_admin' | 'org_user' | null;

interface LoginResponse {
  token?: string;
  user: string;
  role?: string;
  organization_id?: string;
  organization_name?: string;
  requires_2fa?: boolean;
}

interface AuthContextType {
  token: string | null;
  user: string | null;
  role: UserRole;
  organizationId: string | null;
  organizationName: string | null;
  login: (username: string, password: string, otpCode?: string) => Promise<LoginResponse>;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
  isSuperAdmin: boolean;
  isOrgAdmin: boolean;
  isOrgUser: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(true);

  // Lazy initialization - read from localStorage immediately
  const [token, setToken] = useState<string | null>(() => {
    const storedToken = localStorage.getItem('auth_token');
    // Set loading to false after initial load
    setTimeout(() => setIsLoading(false), 0);
    return storedToken;
  });
  const [user, setUser] = useState<string | null>(() => {
    return localStorage.getItem('auth_user');
  });
  const [role, setRole] = useState<UserRole>(() => {
    return (localStorage.getItem('auth_role') as UserRole) || null;
  });
  const [organizationId, setOrganizationId] = useState<string | null>(() => {
    return localStorage.getItem('auth_org_id');
  });
  const [organizationName, setOrganizationName] = useState<string | null>(() => {
    return localStorage.getItem('auth_org_name');
  });

  const login = async (username: string, password: string, otpCode?: string): Promise<LoginResponse> => {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password, otp_code: otpCode }),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Login failed');
    }

    const data: LoginResponse = await response.json();

    // If 2FA is required, return without setting auth state
    if (data.requires_2fa) {
      return data;
    }

    // Set auth state only if we have a token (successful login)
    if (data.token) {
      setToken(data.token);
      setUser(data.user);
      setRole(data.role as UserRole);
      setOrganizationId(data.organization_id || null);
      setOrganizationName(data.organization_name || null);

      localStorage.setItem('auth_token', data.token);
      localStorage.setItem('auth_user', data.user);
      localStorage.setItem('auth_role', data.role || '');
      if (data.organization_id) {
        localStorage.setItem('auth_org_id', data.organization_id);
      }
      if (data.organization_name) {
        localStorage.setItem('auth_org_name', data.organization_name);
      }
    }

    return data;
  };

  const logout = () => {
    setToken(null);
    setUser(null);
    setRole(null);
    setOrganizationId(null);
    setOrganizationName(null);

    localStorage.removeItem('auth_token');
    localStorage.removeItem('auth_user');
    localStorage.removeItem('auth_role');
    localStorage.removeItem('auth_org_id');
    localStorage.removeItem('auth_org_name');
  };

  // Derived properties
  const isSuperAdmin = role === 'super_admin';
  const isOrgAdmin = role === 'org_admin';
  const isOrgUser = role === 'org_user';

  return (
    <AuthContext.Provider
      value={{
        token,
        user,
        role,
        organizationId,
        organizationName,
        login,
        logout,
        isAuthenticated: !!token,
        isLoading,
        isSuperAdmin,
        isOrgAdmin,
        isOrgUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
