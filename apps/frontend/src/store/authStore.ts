import { create } from 'zustand';

interface AuthState {
  user: { id: string; email: string } | null;
  accessToken: string | null;
  setAuth: (user: { id: string; email: string } | null, token: string | null) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  accessToken: null,
  setAuth: (user, token) => {
    if (token) {
      localStorage.setItem('access_token', token);
    } else {
      localStorage.removeItem('access_token');
    }
    set({ user, accessToken: token });
  },
  logout: () => {
    localStorage.removeItem('access_token');
    set({ user: null, accessToken: null });
  },
}));
