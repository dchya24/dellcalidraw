import React, { useState } from 'react';
import { Lock, X } from 'lucide-react';
import { useThemeStore } from '../store/useThemeStore';

interface RoomPasswordDialogProps {
  roomId: string;
  isOpen: boolean;
  onSubmit: (password: string) => void;
  onCancel: () => void;
}

export const RoomPasswordDialog: React.FC<RoomPasswordDialogProps> = ({
  roomId,
  isOpen,
  onSubmit,
  onCancel,
}) => {
  const { theme } = useThemeStore();
  const isDark = theme === 'dark';
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!password.trim()) {
      setError('Please enter a password');
      return;
    }
    setError(null);
    onSubmit(password);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className={`w-full max-w-md rounded-lg shadow-xl ${isDark ? 'bg-gray-800 text-white' : 'bg-white text-gray-900'}`}>
        {/* Header */}
        <div className={`flex items-center justify-between p-4 border-b ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          <div className="flex items-center gap-2">
            <Lock className="w-5 h-5 text-yellow-500" />
            <h2 className="text-lg font-semibold">Password Required</h2>
          </div>
          <button 
            onClick={onCancel} 
            className={`p-1 rounded hover:${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-4">
          <p className={`mb-4 text-sm ${isDark ? 'text-gray-300' : 'text-gray-600'}`}>
            This room is password protected. Please enter the password to join.
          </p>

          <div className={`mb-2 p-2 rounded text-sm ${isDark ? 'bg-gray-700' : 'bg-gray-100'}`}>
            <span className={isDark ? 'text-gray-400' : 'text-gray-500'}>Room ID: </span>
            <span className="font-mono">{roomId}</span>
          </div>

          {error && (
            <div className="mb-4 p-3 bg-red-100 text-red-700 rounded-lg text-sm">
              {error}
            </div>
          )}

          <div className="mb-4">
            <label className={`block text-sm font-medium mb-1 ${isDark ? 'text-gray-300' : 'text-gray-700'}`}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter room password"
              autoFocus
              className={`w-full px-3 py-2 rounded-lg border ${
                isDark 
                  ? 'bg-gray-700 border-gray-600 text-white placeholder-gray-400' 
                  : 'bg-white border-gray-300 text-gray-900 placeholder-gray-500'
              } focus:outline-none focus:ring-2 focus:ring-blue-500`}
            />
          </div>

          <div className="flex gap-3">
            <button
              type="submit"
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              Join Room
            </button>
            <button
              type="button"
              onClick={onCancel}
              className={`px-4 py-2 rounded-lg ${
                isDark 
                  ? 'bg-gray-700 hover:bg-gray-600 text-white' 
                  : 'bg-gray-200 hover:bg-gray-300 text-gray-700'
              } transition-colors`}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default RoomPasswordDialog;
