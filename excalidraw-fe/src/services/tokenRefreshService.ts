import { useAuthStore } from '../store/useAuthStore';

/**
 * Token Refresh Service - Interceptor-based approach
 * 
 * Strategy:
 * 1. Refresh token ONLY when API returns 401 Unauthorized
 * 2. Queue pending requests during refresh
 * 3. Retry failed requests with new token
 * 4. No background polling/timers
 */
class TokenRefreshService {
  private isRefreshing = false;
  private refreshPromise: Promise<boolean> | null = null;
  private pendingRequests: Array<{
    resolve: (value: boolean) => void;
    reject: (error: Error) => void;
  }> = [];

  /**
   * Start the service (no-op for compatibility)
   * Old polling mechanism removed
   */
  start(): void {
    console.log('[TokenRefresh] Service ready (on-demand refresh only)');
  }

  /**
   * Stop the service (no-op for compatibility)
   */
  stop(): void {
    console.log('[TokenRefresh] Service stopped');
    this.isRefreshing = false;
    this.refreshPromise = null;
    this.pendingRequests = [];
  }

  /**
   * Refresh tokens when API returns 401
   * All concurrent calls will wait for the same refresh promise
   */
  async refreshTokens(): Promise<boolean> {
    // If already refreshing, return the existing promise
    if (this.isRefreshing && this.refreshPromise) {
      console.log('[TokenRefresh] Already refreshing, queuing request...');
      return this.refreshPromise;
    }

    const { refreshToken, setAuth, clearAuth } = useAuthStore.getState();

    if (!refreshToken) {
      console.log('[TokenRefresh] No refresh token available');
      return false;
    }

    this.isRefreshing = true;

    // Create a new refresh promise that all concurrent requests will share
    this.refreshPromise = (async () => {
      try {
        console.log('[TokenRefresh] Refreshing tokens...');
        
        // Direct fetch to avoid circular dependency with apiService
        const baseUrl = this.getBaseUrl();
        const response = await fetch(`${baseUrl}/api/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refreshToken }),
        });

        if (!response.ok) {
          throw new Error('Refresh token expired or invalid');
        }

        const data = await response.json();
        
        // Update store with new tokens
        setAuth(data.user, data.accessToken, data.refreshToken);

        console.log('[TokenRefresh] Tokens refreshed successfully');
        
        // Resolve all queued requests
        this.pendingRequests.forEach(({ resolve }) => resolve(true));
        this.pendingRequests = [];
        
        return true;
      } catch (error) {
        console.error('[TokenRefresh] Failed to refresh tokens:', error);

        // Clear auth on refresh failure
        console.log('[TokenRefresh] Refresh failed, clearing auth');
        clearAuth();
        
        // Reject all queued requests
        this.pendingRequests.forEach(({ reject }) => 
          reject(new Error('Token refresh failed'))
        );
        this.pendingRequests = [];

        return false;
      } finally {
        this.isRefreshing = false;
        this.refreshPromise = null;
      }
    })();

    return this.refreshPromise;
  }

  /**
   * Get base URL for API calls
   */
  private getBaseUrl(): string {
    if (typeof window === 'undefined') return 'http://localhost:8080';
    
    const envUrl = import.meta.env.VITE_API_URL;
    if (envUrl) return envUrl;

    const protocol = window.location.protocol === 'https:' ? 'https:' : 'http:';
    const host = window.location.hostname;
    return `${protocol}//${host}:8080`;
  }

  /**
   * Check if current access token is expired (client-side check)
   * Used to proactively refresh before making requests
   */
  isTokenExpired(): boolean {
    const { accessToken } = useAuthStore.getState();
    if (!accessToken) return true;

    try {
      const parts = accessToken.split('.');
      if (parts.length !== 3) return true;

      const payload = JSON.parse(atob(parts[1]));
      if (!payload.exp) return false; // No expiry claim

      const expiryMs = payload.exp * 1000;
      const now = Date.now();
      
      // Consider expired if within 30 seconds of expiry (buffer)
      return expiryMs - now < 30000;
    } catch {
      return true; // Invalid token format
    }
  }
}

// Singleton instance
export const tokenRefreshService = new TokenRefreshService();
