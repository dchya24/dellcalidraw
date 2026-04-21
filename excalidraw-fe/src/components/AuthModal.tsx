import { useState } from "react";
import { X, Loader2, AlertCircle } from "lucide-react";
import { useThemeStore } from "../store/useThemeStore";
import { useAuthStore } from "../store/useAuthStore";
import { apiService, AuthError } from "../services/api";

interface AuthModalProps {
  isOpen: boolean;
  onClose: () => void;
}

type AuthMode = "login" | "register";

export default function AuthModal({ isOpen, onClose }: AuthModalProps) {
  const { theme } = useThemeStore();
  const { setAuth } = useAuthStore();

  const [mode, setMode] = useState<AuthMode>("login");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [username, setUsername] = useState("");

  if (!isOpen) return null;

  const resetForm = () => {
    setEmail("");
    setPassword("");
    setUsername("");
    setError(null);
  };

  const switchMode = (newMode: AuthMode) => {
    setMode(newMode);
    resetForm();
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      let response;
      if (mode === "register") {
        response = await apiService.register({ username, email, password });
      } else {
        response = await apiService.login({ email, password });
      }

      setAuth(
        response.user,
        response.accessToken,
        response.refreshToken
      );

      resetForm();
      onClose();
    } catch (err) {
      if (err instanceof AuthError) {
        if (err.code === "email_taken") {
          setError("Email is already registered");
        } else if (err.code === "username_taken") {
          setError("Username is already taken");
        } else if (err.code === "invalid_credentials") {
          setError("Invalid email or password");
        } else {
          setError(err.message);
        }
      } else {
        setError("Something went wrong. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  };

  const isDark = theme === "dark";

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />

      <div
        className={`relative rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden ${
          isDark
            ? "bg-gray-800 border border-gray-700"
            : "bg-white border border-gray-200"
        }`}
      >
        <div className="p-6">
          <div className="flex items-center justify-between mb-6">
            <h2
              className={`text-xl font-bold ${
                isDark ? "text-white" : "text-gray-900"
              }`}
            >
              {mode === "login" ? "Welcome Back" : "Create Account"}
            </h2>
            <button
              onClick={onClose}
              className={`p-1 rounded-lg transition-colors ${
                isDark
                  ? "hover:bg-gray-700 text-gray-400"
                  : "hover:bg-gray-100 text-gray-500"
              }`}
            >
              <X size={20} />
            </button>
          </div>

          {error && (
            <div
              className={`mb-4 p-3 rounded-lg flex items-center gap-2 text-sm ${
                isDark
                  ? "bg-red-900/30 text-red-400"
                  : "bg-red-50 text-red-600"
              }`}
            >
              <AlertCircle size={16} className="shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {mode === "register" && (
              <div>
                <label
                  className={`block text-sm font-medium mb-1.5 ${
                    isDark ? "text-gray-300" : "text-gray-700"
                  }`}
                >
                  Username
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="At least 3 characters"
                  required
                  minLength={3}
                  className={`w-full px-3 py-2.5 rounded-lg text-sm transition-colors outline-none ${
                    isDark
                      ? "bg-gray-700 text-white placeholder-gray-500 border border-gray-600 focus:border-blue-500"
                      : "bg-gray-50 text-gray-900 placeholder-gray-400 border border-gray-300 focus:border-blue-500"
                  }`}
                />
              </div>
            )}

            <div>
              <label
                className={`block text-sm font-medium mb-1.5 ${
                  isDark ? "text-gray-300" : "text-gray-700"
                }`}
              >
                Email
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                required
                className={`w-full px-3 py-2.5 rounded-lg text-sm transition-colors outline-none ${
                  isDark
                    ? "bg-gray-700 text-white placeholder-gray-500 border border-gray-600 focus:border-blue-500"
                    : "bg-gray-50 text-gray-900 placeholder-gray-400 border border-gray-300 focus:border-blue-500"
                }`}
              />
            </div>

            <div>
              <label
                className={`block text-sm font-medium mb-1.5 ${
                  isDark ? "text-gray-300" : "text-gray-700"
                }`}
              >
                Password
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={mode === "register" ? "At least 8 characters" : "Your password"}
                required
                minLength={mode === "register" ? 8 : 1}
                className={`w-full px-3 py-2.5 rounded-lg text-sm transition-colors outline-none ${
                  isDark
                    ? "bg-gray-700 text-white placeholder-gray-500 border border-gray-600 focus:border-blue-500"
                    : "bg-gray-50 text-gray-900 placeholder-gray-400 border border-gray-300 focus:border-blue-500"
                }`}
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg text-sm font-medium transition-colors flex items-center justify-center gap-2"
            >
              {loading && <Loader2 size={16} className="animate-spin" />}
              {mode === "login" ? "Sign In" : "Create Account"}
            </button>
          </form>

          <div
            className={`mt-5 pt-4 border-t text-center text-sm ${
              isDark ? "border-gray-700" : "border-gray-200"
            }`}
          >
            <span className={isDark ? "text-gray-400" : "text-gray-600"}>
              {mode === "login"
                ? "Don't have an account?"
                : "Already have an account?"}
            </span>
            <button
              onClick={() => switchMode(mode === "login" ? "register" : "login")}
              className="ml-1.5 text-blue-500 hover:text-blue-400 font-medium transition-colors"
            >
              {mode === "login" ? "Sign Up" : "Sign In"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
