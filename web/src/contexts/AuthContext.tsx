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
  user: string | null;
  role: UserRole;
  organizationId: string | null;
  organizationName: string | null;
  csrfToken: string | null;
  login: (username: string, password: string, otpCode?: string) => Promise<LoginResponse>;
  logout: () => Promise<void>;
  isAuthenticated: boolean;
  isLoading: boolean;
  isSuperAdmin: boolean;
  isOrgAdmin: boolean;
  isOrgUser: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(true);
  const [csrfToken, setCSRFToken] = useState<string | null>(null);

  // Use sessionStorage for user info (not sensitive, XSS safe enough)
  const [user, setUser] = useState<string | null>(() => {
    setTimeout(() => setIsLoading(false), 0);
    return sessionStorage.getItem('auth_user');
  });
  const [role, setRole] = useState<UserRole>(() => {
    return (sessionStorage.getItem('auth_role') as UserRole) || null;
  });
  const [organizationId, setOrganizationId] = useState<string | null>(() => {
    return sessionStorage.getItem('auth_org_id');
  });
  const [organizationName, setOrganizationName] = useState<string | null>(() => {
    return sessionStorage.getItem('auth_org_name');
  });

  const login = async (username: string, password: string, otpCode?: string): Promise<LoginResponse> => {
    const response = await fetch('/api/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include', // Important: include cookies
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

    // After successful login, fetch CSRF token
    const csrfResponse = await fetch('/api/auth/csrf', {
      credentials: 'include',
    });
    if (csrfResponse.ok) {
      const csrfData = await csrfResponse.json();
      setCSRFToken(csrfData.csrf_token);
      // Set global CSRF token for API client
      window._csrfToken = csrfData.csrf_token;
    }

    // Set auth state (cookie is set automatically by server)
    setUser(data.user);
    setRole(data.role as UserRole);
    setOrganizationId(data.organization_id || null);
    setOrganizationName(data.organization_name || null);

    // Store non-sensitive data in sessionStorage
    sessionStorage.setItem('auth_user', data.user);
    sessionStorage.setItem('auth_role', data.role || '');
    if (data.organization_id) {
      sessionStorage.setItem('auth_org_id', data.organization_id);
    }
    if (data.organization_name) {
      sessionStorage.setItem('auth_org_name', data.organization_name);
    }

    return data;
  };

  const logout = async () => {
    // Call logout endpoint to clear cookie
    await fetch('/api/auth/logout', {
      method: 'POST',
      credentials: 'include',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });

    // Clear state
    setUser(null);
    setRole(null);
    setOrganizationId(null);
    setOrganizationName(null);
    setCSRFToken(null);
    window._csrfToken = null;

    sessionStorage.removeItem('auth_user');
    sessionStorage.removeItem('auth_role');
    sessionStorage.removeItem('auth_org_id');
    sessionStorage.removeItem('auth_org_name');
  };

  // Derived properties
  const isSuperAdmin = role === 'super_admin';
  const isOrgAdmin = role === 'org_admin';
  const isOrgUser = role === 'org_user';

  return (
    <AuthContext.Provider
      value={{
        user,
        role,
        organizationId,
        organizationName,
        csrfToken,
        login,
        logout,
        isAuthenticated: !!user, // User presence indicates authentication
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
