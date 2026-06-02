# Token Refresh Strategy

## 🎯 Overview

**New Strategy (2026-06-02)**: Interceptor-based refresh on 401 responses
**Old Strategy (Removed)**: Polling every 30 seconds

---

## ✅ How It Works

### 1. **API Services with Auto-Retry**
- `apiService.ts` - All auth/file/canvas operations
- `fileService.ts` - File CRUD operations
- Both services intercept 401 responses and trigger token refresh

### 2. **Token Refresh Flow**

```
┌─────────────┐
│ API Request │
│ (with token)│
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  401 Error? │────No────▶ Return response
└──────┬──────┘
       │ Yes
       ▼
┌─────────────────────┐
│ tokenRefreshService │
│  .refreshTokens()   │
└──────┬──────────────┘
       │
       ├──Success──▶ Retry request with new token
       │
       └──Failure──▶ clearAuth() + throw error
```

### 3. **Concurrency Handling**
- Multiple 401s trigger only **1 refresh request**
- Subsequent calls wait for the same refresh promise
- No race conditions

### 4. **AI Service Proactive Check**
- Before AI streaming: check if token expired (JWT exp claim)
- Refresh preemptively if within 30s of expiry
- Prevents mid-stream authentication errors

---

## 📂 Updated Files

| File | Change |
|------|--------|
| `tokenRefreshService.ts` | Removed polling, added promise queueing |
| `api.ts` | Added 401 interceptor with retry |
| `fileService.ts` | Added 401 interceptor with retry |
| `ai/aiService.ts` | Added proactive token expiry check |
| `App.tsx` | No changes (start/stop are no-ops now) |

---

## 🔧 Implementation Details

### TokenRefreshService

```typescript
class TokenRefreshService {
  private isRefreshing = false;
  private refreshPromise: Promise<boolean> | null = null;

  async refreshTokens(): Promise<boolean> {
    // Deduplication: return existing promise if already refreshing
    if (this.isRefreshing && this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = (async () => {
      // Direct fetch to avoid circular dependency
      const response = await fetch('/api/auth/refresh', { ... });
      
      if (response.ok) {
        setAuth(newTokens);
        return true;
      } else {
        clearAuth(); // Force logout
        return false;
      }
    })();

    return this.refreshPromise;
  }

  isTokenExpired(): boolean {
    // Decode JWT exp claim
    // Return true if expires within 30s
  }
}
```

### API Service Interceptor

```typescript
private async request<T>(path, options, retryOn401 = true): Promise<T> {
  const response = await fetch(url, { headers: { Authorization } });

  if (response.status === 401 && retryOn401) {
    const refreshSuccess = await tokenRefreshService.refreshTokens();
    
    if (refreshSuccess) {
      return this.request<T>(path, options, false); // Retry once
    } else {
      throw new AuthError('Session expired', 'token_expired', 401);
    }
  }

  return response.json();
}
```

---

## 🚀 Benefits Over Old Strategy

| Aspect | Old (Polling) | New (Interceptor) |
|--------|---------------|-------------------|
| Network calls | Every 30s regardless of activity | Only when 401 occurs |
| Token lifetime | Max 30s before refresh | Full token lifetime used |
| Race conditions | Possible (timer + API call) | Prevented (promise dedup) |
| Battery impact | High (background polling) | Low (reactive only) |
| Code complexity | Medium (timers + expiry tracking) | Low (interceptor only) |

---

## 🧪 Testing

### Manual Test
1. Login to app
2. Wait for token to expire (check JWT exp claim)
3. Make any API call (save file, join room, etc.)
4. **Expected**: 401 → auto-refresh → request succeeds
5. Check console: `[TokenRefresh] Tokens refreshed successfully`

### Edge Cases Covered
- ✅ Concurrent 401s (multiple API calls expire simultaneously)
- ✅ Refresh token expired → auto-logout
- ✅ Network error during refresh → logout
- ✅ AI streaming with expired token → proactive refresh before stream

---

## 🔒 Security Notes

1. **Refresh endpoint excluded from retry** - prevents infinite loop if refresh itself returns 401
2. **Direct fetch in tokenRefreshService** - avoids circular dependency with apiService
3. **Auto-logout on refresh failure** - prevents stuck "authenticated but can't access" state
4. **Token expiry buffer (30s)** - prevents mid-operation expiry

---

## 📊 Performance Metrics

**Before (Polling every 30s for 1 hour session):**
- Network requests: 120 refresh checks
- Wasted calls: ~118 (if token lifetime is 15min)

**After (Reactive on 401):**
- Network requests: 4-5 refresh calls (1 per token expiry)
- Wasted calls: 0

**Savings: ~97% reduction in refresh-related network traffic**

---

## 🛠️ Migration Notes

- `start()` and `stop()` are now no-ops for backward compatibility
- `forceRefresh()` removed (unused)
- `setTokenExpiry()` removed (JWT exp claim used instead)
- `getTimeUntilExpiry()` removed (not needed for interceptor pattern)

---

## 📝 Future Improvements

1. **WebSocket token refresh**: Handle WS connection re-auth on token expiry
2. **Retry with exponential backoff**: If refresh fails due to network (not auth error)
3. **Token rotation in background**: Proactive refresh at 80% of token lifetime (optional)
4. **Refresh token rotation**: Backend rotates refresh token on each use (security best practice)
