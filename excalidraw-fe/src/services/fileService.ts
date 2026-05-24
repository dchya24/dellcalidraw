import type { ApiError } from "../types/auth";

function getBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_URL;
  if (envUrl) return envUrl;

  const protocol = window.location.protocol === "https:" ? "https:" : "http:";
  const host = window.location.hostname;
  return `${protocol}//${host}:8080`;
}

// User file types (frontend representation)
export interface FileTab {
  tabKey: string;
  title: string;
  roomId: string;
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}

export interface UserFile {
  id: string;
  userId: string;
  name: string;
  tabCount: number;
  createdAt: string;
  updatedAt: string;
  tabs?: FileTab[];
}

class FileService {
  private baseUrl: string;

  constructor() {
    this.baseUrl = getBaseUrl();
  }

  private async request<T>(
    path: string,
    options: RequestInit = {}
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

    if (!response.ok) {
      let error: ApiError;
      try {
        error = await response.json();
      } catch {
        error = { message: "Request failed", code: "unknown_error" };
      }
      throw new Error(error.message);
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

  // List all files for the authenticated user
  async listFiles(): Promise<{ files: UserFile[]; count: number }> {
    return this.request("/api/files");
  }

  // Create a new file
  async createFile(name: string = "Untitled"): Promise<{ file: UserFile }> {
    return this.request("/api/files", {
      method: "POST",
      body: JSON.stringify({ name }),
    });
  }

  // Get a specific file
  async getFile(fileId: string): Promise<{ file: UserFile }> {
    return this.request(`/api/files/${fileId}`);
  }

  // Update file metadata
  async updateFile(fileId: string, data: { name?: string; tabCount?: number }): Promise<{ file: UserFile }> {
    return this.request(`/api/files/${fileId}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Rename a file
  async renameFile(fileId: string, name: string): Promise<{ fileId: string; name: string }> {
    return this.request(`/api/files/${fileId}/rename`, {
      method: "PATCH",
      body: JSON.stringify({ name }),
    });
  }

  // Delete a file
  async deleteFile(fileId: string): Promise<{ fileId: string }> {
    return this.request(`/api/files/${fileId}`, {
      method: "DELETE",
    });
  }

  // Migrate local files to cloud
  async migrateFiles(files: Array<{
    name: string;
    activeTabId: string;
    tabs: Array<{
      title: string;
      roomId: string;
      elements: unknown[];
      appState: Record<string, unknown>;
      files: Record<string, unknown>;
    }>;
  }>): Promise<{ files: Array<UserFile & { tabs: FileTab[] }>; count: number }> {
    return this.request("/api/files/migrate", {
      method: "POST",
      body: JSON.stringify({ files }),
    });
  }

  // Save/update all tabs for a file
  async saveFileTabs(fileId: string, tabs: Array<{
    tabKey: string;
    title: string;
    roomId: string;
    elements: unknown[];
    appState: Record<string, unknown>;
    files: Record<string, unknown>;
  }>): Promise<{ success: boolean; count: number }> {
    return this.request(`/api/files/${fileId}/tabs`, {
      method: "PUT",
      body: JSON.stringify({ tabs }),
    });
  }
}

export const fileService = new FileService();