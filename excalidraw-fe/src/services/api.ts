import type {
  AuthResponse,
  RegisterRequest,
  LoginRequest,
  ApiError,
} from "../types/auth";

function getBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) return envUrl;

  const protocol = window.location.protocol === "https:" ? "https:" : "http:";
  const host = window.location.hostname;
  return `${protocol}//${host}:8080`;
}

class ApiService {
  private baseUrl: string;

  constructor() {
    this.baseUrl = getBaseUrl();
  }

  private async request<T>(
    path: string,
    options: RequestInit = {},
    retryOn401 = true
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...((options.headers as Record<string, string>) || {}),
    };

    const token = this.getStoredAccessToken();
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    const response = await fetch(url, {
      ...options,
      headers,
    });

    // Handle 401 Unauthorized - trigger token refresh
    if (response.status === 401 && retryOn401 && path !== '/api/auth/refresh') {
      console.log('[ApiService] 401 Unauthorized, attempting token refresh...');

      // Dynamic import to avoid circular dependency
      const { tokenRefreshService } = await import('./tokenRefreshService');
      const refreshSuccess = await tokenRefreshService.refreshTokens();

      if (refreshSuccess) {
        console.log('[ApiService] Token refreshed, retrying request...');
        // Retry the request once with new token (retryOn401 = false to prevent infinite loop)
        return this.request<T>(path, options, false);
      } else {
        console.log('[ApiService] Token refresh failed, request aborted');
        throw new AuthError('Session expired', 'token_expired', 401);
      }
    }

    if (!response.ok) {
      let error: ApiError;
      try {
        error = await response.json();
      } catch {
        error = { message: "Request failed", code: "unknown_error" };
      }
      throw new AuthError(error.message, error.code, response.status);
    }

    return response.json();
  }

  private getStoredAccessToken(): string | null {
    try {
      const raw = localStorage.getItem("auth-storage");
      if (!raw) return null;
      const parsed = JSON.parse(raw);
      return parsed?.state?.accessToken || null;
    } catch {
      return null;
    }
  }

  async register(data: RegisterRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    }, false);
  }

  async login(data: LoginRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }, false);
  }

  async refreshToken(refreshToken: string): Promise<AuthResponse> {
    return this.request<AuthResponse>("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refreshToken }),
    });
  }

  async logout(refreshToken: string): Promise<void> {
    await this.request("/api/auth/logout", {
      method: "POST",
      body: JSON.stringify({ refreshToken }),
    });
  }

  // Password Reset Methods (Phase 12)
  async forgotPassword(email: string): Promise<{ success: boolean; message: string }> {
    return this.request("/api/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
  }

  async validateResetToken(token: string): Promise<{ valid: boolean; email: string; expiresAt: string }> {
    return this.request("/api/auth/validate-reset-token", {
      method: "POST",
      body: JSON.stringify({ token }),
    });
  }

  async resetPassword(token: string, newPassword: string): Promise<{ success: boolean; message: string }> {
    return this.request("/api/auth/reset-password", {
      method: "POST",
      body: JSON.stringify({ token, newPassword }),
    });
  }

  // Canvas Save/Load Methods
  async saveCanvas(roomId: string): Promise<{ success: boolean; message: string; count: number }> {
    return this.request(`/api/rooms/${roomId}/canvas/save`, {
      method: "POST",
    });
  }

  async loadCanvas(roomId: string): Promise<{ success: boolean; elements: unknown[]; count: number }> {
    return this.request(`/api/rooms/${roomId}/canvas/load`, {
      method: "GET",
    });
  }

  async restoreCanvas(roomId: string): Promise<{ success: boolean; message: string; count: number }> {
    return this.request(`/api/rooms/${roomId}/canvas/restore`, {
      method: "POST",
    });
  }

  async clearCanvas(roomId: string): Promise<{ success: boolean; message: string; count: number }> {
    return this.request(`/api/rooms/${roomId}/canvas`, {
      method: "DELETE",
    });
  }
}

export class AuthError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "AuthError";
    this.code = code;
    this.status = status;
  }
}

export const apiService = new ApiService();
