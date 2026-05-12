import { useAuthStore } from '../store/useAuthStore';
import { apiService } from './api';

// Token refresh configuration
const TOKEN_REFRESH_THRESHOLD_MS = 2 * 60 * 1000; // Refresh 2 minutes before expiry
const TOKEN_CHECK_INTERVAL_MS = 30 * 1000; // Check every 30 seconds

class TokenRefreshService {
  private refreshTimer: ReturnType<typeof setInterval> | null = null;
  private isRefreshing = false;
  private tokenExpiresAt: number | null = null;

  /**
   * Start the auto-refresh service
   */
  start(): void {
    if (this.refreshTimer) {
      return; // Already running
    }

    console.log('[TokenRefresh] Starting auto-refresh service');

    // Initial check
    this.checkAndRefresh();

    // Set up periodic check
    this.refreshTimer = setInterval(() => {
      this.checkAndRefresh();
    }, TOKEN_CHECK_INTERVAL_MS);
  }

  /**
   * Stop the auto-refresh service
   */
  stop(): void {
    if (this.refreshTimer) {
      console.log('[TokenRefresh] Stopping auto-refresh service');
      clearInterval(this.refreshTimer);
      this.refreshTimer = null;
    }
    this.tokenExpiresAt = null;
  }

  /**
   * Set the token expiration time (called after login/refresh)
   */
  setTokenExpiry(expiresAt: Date | string): void {
    const expiry = typeof expiresAt === 'string' ? new Date(expiresAt) : expiresAt;
    this.tokenExpiresAt = expiry.getTime();
    console.log('[TokenRefresh] Token expires at:', expiry.toISOString());
  }

  /**
   * Check if token needs refresh and refresh if necessary
   */
  private async checkAndRefresh(): Promise<void> {
    const { isAuthenticated, refreshToken, accessToken } = useAuthStore.getState();

    if (!isAuthenticated || !refreshToken || !accessToken) {
      return;
    }

    // Try to get expiry from stored value or decode from token
    if (!this.tokenExpiresAt) {
      this.tokenExpiresAt = this.getExpiryFromToken(accessToken);
    }

    if (!this.tokenExpiresAt) {
      return;
    }

    const now = Date.now();
    const timeUntilExpiry = this.tokenExpiresAt - now;

    // Check if we need to refresh
    if (timeUntilExpiry <= TOKEN_REFRESH_THRESHOLD_MS) {
      console.log('[TokenRefresh] Token expiring soon, refreshing...');
      await this.refreshTokens();
    }
  }

  /**
   * Refresh the tokens
   */
  private async refreshTokens(): Promise<boolean> {
    if (this.isRefreshing) {
      console.log('[TokenRefresh] Already refreshing, skipping');
      return false;
    }

    const { refreshToken, setAuth, clearAuth } = useAuthStore.getState();

    if (!refreshToken) {
      console.log('[TokenRefresh] No refresh token available');
      return false;
    }

    this.isRefreshing = true;

    try {
      console.log('[TokenRefresh] Refreshing tokens...');
      const response = await apiService.refreshToken(refreshToken);

      // Update store with new tokens
      setAuth(response.user, response.accessToken, response.refreshToken);

      // Update expiry
      this.setTokenExpiry(response.expiresAt);

      console.log('[TokenRefresh] Tokens refreshed successfully');
      return true;
    } catch (error) {
      console.error('[TokenRefresh] Failed to refresh tokens:', error);

      // If refresh fails (e.g., token revoked), clear auth
      if (error instanceof Error && error.message.includes('invalid')) {
        console.log('[TokenRefresh] Refresh token invalid, clearing auth');
        clearAuth();
        this.stop();
      }

      return false;
    } finally {
      this.isRefreshing = false;
    }
  }

  /**
   * Extract expiry time from JWT token
   */
  private getExpiryFromToken(token: string): number | null {
    try {
      const parts = token.split('.');
      if (parts.length !== 3) {
        return null;
      }

      const payload = JSON.parse(atob(parts[1]));
      if (payload.exp) {
        return payload.exp * 1000; // Convert to milliseconds
      }

      return null;
    } catch {
      return null;
    }
  }

  /**
   * Force an immediate token refresh
   */
  async forceRefresh(): Promise<boolean> {
    return this.refreshTokens();
  }

  /**
   * Get time until token expires (in milliseconds)
   */
  getTimeUntilExpiry(): number | null {
    if (!this.tokenExpiresAt) {
      const { accessToken } = useAuthStore.getState();
      if (accessToken) {
        this.tokenExpiresAt = this.getExpiryFromToken(accessToken);
      }
    }

    if (!this.tokenExpiresAt) {
      return null;
    }

    return Math.max(0, this.tokenExpiresAt - Date.now());
  }

  /**
   * Check if token is expired
   */
  isTokenExpired(): boolean {
    const timeUntilExpiry = this.getTimeUntilExpiry();
    return timeUntilExpiry !== null && timeUntilExpiry <= 0;
  }
}

// Singleton instance
export const tokenRefreshService = new TokenRefreshService();
