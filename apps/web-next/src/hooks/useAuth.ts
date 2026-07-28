"use client";

import { createContext, useContext } from "react";
import type { User } from "@/lib/types";

export interface AuthContextType {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  /** Exchange a magic-link code for a session. */
  verifyCode: (code: string) => Promise<void>;
  logout: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: true,
  login: async () => {},
  verifyCode: async () => {},
  logout: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}
