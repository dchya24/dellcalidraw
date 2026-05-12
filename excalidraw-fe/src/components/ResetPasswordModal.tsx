import React, { useState, useEffect } from 'react';
import { X, Lock, CheckCircle, AlertCircle, Loader2 } from 'lucide-react';
import { useThemeStore } from '../store/useThemeStore';
import { apiService, AuthError } from '../services/api';

interface ResetPasswordModalProps {
  isOpen: boolean;
  token: string;
  onClose: () => void;
  onSuccess: () => void;
}

export const ResetPasswordModal: React.FC<ResetPasswordModalProps> = ({
  isOpen,
  token,
  onClose,
  onSuccess,
}) => {
  const { theme } = useThemeStore();
  const isDark = theme === 'dark';

  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [validating, setValidating] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [tokenValid, setTokenValid] = useState(false);
  const [maskedEmail, setMaskedEmail] = useState('');

  // Validate token on mount
  useEffect(() => {
    if (!isOpen || !token) return;

    const validateToken = async () => {
      setValidating(true);
      setError(null);

      try {
        const result = await apiService.validateResetToken(token);
        setTokenValid(result.valid);
        setMaskedEmail(result.email || '');
      } catch (err) {
        setTokenValid(false);
        if (err instanceof AuthError) {
          setError(err.message);
        } else {
          setError('Invalid or expired reset link');
        }
      } finally {
        setValidating(false);
      }
    };

    validateToken();
  }, [isOpen, token]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (newPassword.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    if (newPassword !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    setLoading(true);

    try {
      await apiService.resetPassword(token, newPassword);
      setSuccess(true);
    } catch (err) {
      if (err instanceof AuthError) {
        setError(err.message);
      } else {
        setError('Failed to reset password. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setNewPassword('');
    setConfirmPassword('');
    setError(null);
    setSuccess(false);
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className={`w-full max-w-md rounded-lg shadow-xl ${isDark ? 'bg-gray-800 text-white' : 'bg-white text-gray-900'}`}>
        {/* Header */}
        <div className={`flex items-center justify-between p-4 border-b ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          <div className="flex items-center gap-2">
            <Lock className="w-5 h-5" />
            <h2 className="text-lg font-semibold">Set New Password</h2>
          </div>
          <button
            onClick={handleClose}
            className={`p-1 rounded hover:${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {validating ? (
            <div className="text-center py-8">
              <Loader2 className={`w-8 h-8 mx-auto mb-4 animate-spin ${isDark ? 'text-blue-400' : 'text-blue-600'}`} />
              <p className={isDark ? 'text-gray-400' : 'text-gray-600'}>Validating reset link...</p>
            </div>
          ) : !tokenValid ? (
            <div className="text-center">
              <div className={`mx-auto w-16 h-16 rounded-full flex items-center justify-center mb-4 ${isDark ? 'bg-red-900/30' : 'bg-red-100'}`}>
                <AlertCircle className={`w-8 h-8 ${isDark ? 'text-red-400' : 'text-red-600'}`} />
              </div>
              <h3 className="text-lg font-semibold mb-2">Invalid Reset Link</h3>
              <p className={`text-sm mb-6 ${isDark ? 'text-gray-400' : 'text-gray-600'}`}>
                {error || 'This password reset link is invalid or has expired. Please request a new one.'}
              </p>
              <button
                onClick={handleClose}
                className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
              >
                Close
              </button>
            </div>
          ) : success ? (
            <div className="text-center">
              <div className={`mx-auto w-16 h-16 rounded-full flex items-center justify-center mb-4 ${isDark ? 'bg-green-900/30' : 'bg-green-100'}`}>
                <CheckCircle className={`w-8 h-8 ${isDark ? 'text-green-400' : 'text-green-600'}`} />
              </div>
              <h3 className="text-lg font-semibold mb-2">Password Reset Complete</h3>
              <p className={`text-sm mb-6 ${isDark ? 'text-gray-400' : 'text-gray-600'}`}>
                Your password has been successfully reset. You can now sign in with your new password.
              </p>
              <button
                onClick={onSuccess}
                className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
              >
                Sign In
              </button>
            </div>
          ) : (
            <form onSubmit={handleSubmit}>
              {maskedEmail && (
                <p className={`text-sm mb-4 ${isDark ? 'text-gray-400' : 'text-gray-600'}`}>
                  Enter a new password for <strong>{maskedEmail}</strong>
                </p>
              )}

              {error && (
                <div className={`mb-4 p-3 rounded-lg flex items-center gap-2 ${isDark ? 'bg-red-900/30 text-red-400' : 'bg-red-100 text-red-700'}`}>
                  <AlertCircle className="w-4 h-4 flex-shrink-0" />
                  <span className="text-sm">{error}</span>
                </div>
              )}

              <div className="mb-4">
                <label className={`block text-sm font-medium mb-1 ${isDark ? 'text-gray-300' : 'text-gray-700'}`}>
                  New Password
                </label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="At least 8 characters"
                  autoFocus
                  className={`w-full px-3 py-2 rounded-lg border ${
                    isDark
                      ? 'bg-gray-700 border-gray-600 text-white placeholder-gray-400'
                      : 'bg-white border-gray-300 text-gray-900 placeholder-gray-500'
                  } focus:outline-none focus:ring-2 focus:ring-blue-500`}
                />
              </div>

              <div className="mb-6">
                <label className={`block text-sm font-medium mb-1 ${isDark ? 'text-gray-300' : 'text-gray-700'}`}>
                  Confirm Password
                </label>
                <input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Re-enter your password"
                  className={`w-full px-3 py-2 rounded-lg border ${
                    isDark
                      ? 'bg-gray-700 border-gray-600 text-white placeholder-gray-400'
                      : 'bg-white border-gray-300 text-gray-900 placeholder-gray-500'
                  } focus:outline-none focus:ring-2 focus:ring-blue-500`}
                />
              </div>

              <button
                type="submit"
                disabled={loading}
                className="w-full py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                {loading ? 'Resetting...' : 'Reset Password'}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};

export default ResetPasswordModal;
